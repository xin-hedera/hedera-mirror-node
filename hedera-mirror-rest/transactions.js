/*-
 * ‌
 * Hedera Mirror Node
 * ​
 * Copyright (C) 2019 - 2020 Hedera Hashgraph, LLC
 * ​
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * ‍
 */

'use strict';

const utils = require('./utils');
const constants = require('./constants');
const EntityId = require('./entityId');
const TransactionId = require('./transactionId');
const {DbError} = require('./errors/dbError');
const {NotFoundError} = require('./errors/notFoundError');

/**
 * Gets the select clause with token transfers sorted by token_id and account_id in the specified order
 *
 * @param {string} order sorting order
 * @return {string}
 */
const getSelectClauseWithTokenTransferOrder = (order) => {
  // token transfers are aggregated as an array of json objects {token_id, account_id, amount}
  return `SELECT
       t.payer_account_id,
       t.memo,
       t.consensus_ns,
       t.valid_start_ns,
       coalesce(ttr.result, 'UNKNOWN') AS result,
       coalesce(ttt.name, 'UNKNOWN') AS name,
       t.node_account_id,
       ctl.entity_id AS ctl_entity_id,
       ctl.amount AS amount,
       json_agg(
         json_build_object(
           'token_id', ttl.token_id::text,
           'account_id', ttl.account_id::text,
           'amount', ttl.amount
         ) ORDER BY
             ttl.token_id ${order || ''},
             ttl.account_id ${order || ''}
       ) FILTER (WHERE ttl.token_id IS NOT NULL) AS token_transfer_list,
       t.charged_tx_fee,
       t.valid_duration_seconds,
       t.max_fee,
       t.transaction_hash`;
};

/**
 * Creates token transfer list from aggregated array of JSON objects in the query result
 *
 * @param tokenTransferList token transfer list string
 * @return {undefined|{amount: Number, account: string, token_id: string}[]}
 */
const createTokenTransferList = (tokenTransferList) => {
  if (!tokenTransferList) {
    return undefined;
  }

  return tokenTransferList.map((transfer) => {
    const {token_id: tokenId, account_id: accountId, amount} = transfer;
    return {
      token_id: EntityId.fromString(tokenId).toString(),
      account: EntityId.fromString(accountId).toString(),
      amount,
    };
  });
};

/**
 * Create transferlists from the output of SQL queries. The SQL table has different
 * rows for each of the transfers in a single transaction. This function collates all
 * transfers into a single list.
 * @param {Array of objects} rows Array of rows returned as a result of an SQL query
 * @return {{anchorSecNs: (String|number), transactions: {}}}
 */
const createTransferLists = (rows) => {
  // If the transaction has a transferlist (i.e. list of individual trasnfers, it
  // will show up as separate rows. Combine those into a single transferlist for
  // a given consensus_ns (Note that there could be two records for the same
  // transaction-id where one would pass and others could fail as duplicates)
  const transactions = {};

  for (const row of rows) {
    if (!(row.consensus_ns in transactions)) {
      const validStartTimestamp = row.valid_start_ns;
      transactions[row.consensus_ns] = {
        consensus_timestamp: utils.nsToSecNs(row.consensus_ns),
        transaction_hash: utils.encodeBase64(row.transaction_hash),
        valid_start_timestamp: utils.nsToSecNs(validStartTimestamp),
        charged_tx_fee: Number(row.charged_tx_fee),
        id: row.id,
        memo_base64: utils.encodeBase64(row.memo),
        result: row.result,
        name: row.name,
        max_fee: utils.getNullableNumber(row.max_fee),
        valid_duration_seconds: utils.getNullableNumber(row.valid_duration_seconds),
        node: EntityId.fromString(row.node_account_id).toString(),
        transaction_id: utils.createTransactionId(
          EntityId.fromString(row.payer_account_id).toString(),
          validStartTimestamp
        ),
        transfers: [],
        token_transfers: createTokenTransferList(row.token_transfer_list),
      };
    }

    if (row.ctl_entity_id !== null) {
      transactions[row.consensus_ns].transfers.push({
        account: EntityId.fromString(row.ctl_entity_id).toString(),
        amount: Number(row.amount),
      });
    }
  }

  const anchorSecNs = rows.length > 0 ? utils.nsToSecNs(rows[rows.length - 1].consensus_ns) : 0;

  return {
    transactions: Object.values(transactions),
    anchorSecNs,
  };
};

/**
 * Transactions queries are organized as follows: First there's an inner query that selects the
 * required number of unique transactions (identified by consensus_timestamp). And then queries other tables to
 * extract all relevant information for those transactions.
 * This function returns the outer query base on the consensus_timestamps list returned by the inner query.
 * Also see: getTransactionsInnerQuery function
 * @param {String} innerQuery SQL query that provides a list of unique transactions that match the query criteria
 * @param {String} order Sorting order
 * @return {String} outerQuery Fully formed SQL query
 */
const getTransactionsOuterQuery = function (innerQuery, order) {
  return `
    ${getSelectClauseWithTokenTransferOrder(order)}
    FROM ( ${innerQuery} ) AS tlist
       JOIN transaction t ON tlist.consensus_timestamp = t.consensus_ns
       LEFT OUTER JOIN t_transaction_results ttr ON ttr.proto_id = t.result
       LEFT OUTER JOIN t_transaction_types ttt ON ttt.proto_id = t.type
       LEFT OUTER JOIN crypto_transfer ctl ON tlist.consensus_timestamp = ctl.consensus_timestamp
       LEFT OUTER JOIN token_transfer ttl
         ON t.type = ${constants.transactionTypes.CRYPTOTRANSFER}
         AND tlist.consensus_timestamp = ttl.consensus_timestamp
     GROUP BY t.consensus_ns, ctl_entity_id, ctl.amount, ttr.result, ttt.name, t.payer_account_id, t.memo, t.valid_start_ns, t.node_account_id, t.charged_tx_fee, t.valid_duration_seconds, t.max_fee, t.transaction_hash
     ORDER BY t.consensus_ns ${order} , ctl_entity_id ASC, amount ASC`;
};

/**
 * Build the where clause from an array of query conditions
 *
 * @param {string} conditions Query conditions
 * @return {string} The where clause built from the query conditions
 */
const buildWhereClause = function (...conditions) {
  const clause = conditions.filter((q) => q !== '').join(' AND ');
  return clause === '' ? '' : `WHERE ${clause}`;
};

/**
 * Convert parameters to the named format.
 *
 * @param {String} query - the mysql query
 * @param {String} prefix - the prefix for the named parameters
 * @return {String} - The converted query
 */
const convertToNamedQuery = function (query, prefix) {
  let index = 0;
  return query.replace(/\?/g, () => {
    const namedParam = `?${prefix}${index}`;
    index += 1;
    return namedParam;
  });
};

/**
 * Get the transactions inner query for transactions with a crypto transfer list matching the query filters.
 *
 * @param namedAccountQuery - Account query with named parameters
 * @param namedTsQuery - Transaction table timestamp query with named parameters
 * @param resultTypeQuery - Transaction result query
 * @param limitQuery - Limit query
 * @param creditDebitQuery - Credit or debit query
 * @param transactionTypeQuery - Transaction type query
 * @param order - Sorting order
 * @return {string} - The inner query string
 */
const getCryptoTransferTransactionsInnerQuery = function (
  namedAccountQuery,
  namedTsQuery,
  resultTypeQuery,
  limitQuery,
  creditDebitQuery,
  transactionTypeQuery,
  order
) {
  const namedCtlTsQuery = namedTsQuery.replace(/t\.consensus_ns/g, 'ctl.consensus_timestamp');
  const ctlWhereClause = buildWhereClause(namedAccountQuery, namedCtlTsQuery, creditDebitQuery);
  const whereClause = buildWhereClause(namedTsQuery, resultTypeQuery, transactionTypeQuery);
  return `
      SELECT t.consensus_ns AS consensus_timestamp
      FROM transaction t
      JOIN (
        SELECT DISTINCT consensus_timestamp
        FROM crypto_transfer AS ctl
        ${ctlWhereClause}
        ORDER BY consensus_timestamp ${order}
      ) AS ctl
      ON t.consensus_ns = ctl.consensus_timestamp
      ${whereClause}
      ORDER BY t.consensus_ns ${order}
      ${limitQuery}`;
};

/**
 * Get the general transactions inner query for transactions matching the query filters.
 *
 * @param namedAccountQuery - Account query with named parameters
 * @param namedTsQuery - Transaction table timestamp query with named parameters
 * @param resultTypeQuery - Transaction result query
 * @param namedLimitQuery - Limit query with named parameters
 * @param transactionTypeQuery - Transaction type query
 * @param order - Sorting order
 * @return {string} - The inner query string
 */
const getGeneralTransactionsInnerQuery = function (
  namedAccountQuery,
  namedTsQuery,
  resultTypeQuery,
  namedLimitQuery,
  transactionTypeQuery,
  order
) {
  const transactionAccountQuery = namedAccountQuery.replace(/ctl\.entity_id/g, 't.payer_account_id');
  const transactionWhereClause = buildWhereClause(
    transactionAccountQuery,
    namedTsQuery,
    resultTypeQuery,
    transactionTypeQuery
  );
  const transactionOnlyQuery = `
    SELECT consensus_ns AS consensus_timestamp
    FROM transaction AS t
    ${transactionWhereClause}
    ORDER BY consensus_ns ${order}
    ${namedLimitQuery}`;

  if (namedAccountQuery) {
    // account filter applies to both transaction.payer_account_id and crypto_transfer.entity_id, a full outer join is
    // needed to get rows not in the other table.
    const namedCtlTsQuery = namedTsQuery.replace(/t\.consensus_ns/g, 'ctl.consensus_timestamp');
    const ctlJoinClause =
      (resultTypeQuery || transactionTypeQuery) && 'JOIN transaction AS t ON ctl.consensus_timestamp = t.consensus_ns';
    const ctlWhereClause = buildWhereClause(namedAccountQuery, namedCtlTsQuery, resultTypeQuery, transactionTypeQuery);
    const ctlQuery = `
      SELECT DISTINCT ctl.consensus_timestamp AS consensus_timestamp
        FROM crypto_transfer AS ctl
        ${ctlJoinClause}
        ${ctlWhereClause}
        ORDER BY ctl.consensus_timestamp ${order}
        ${namedLimitQuery}`;

    return `
      SELECT coalesce(t.consensus_timestamp,ctl.consensus_timestamp) AS consensus_timestamp
      FROM (${transactionOnlyQuery}) AS t
      FULL OUTER JOIN (${ctlQuery}) AS ctl
      ON t.consensus_timestamp = ctl.consensus_timestamp
      ORDER BY consensus_timestamp ${order}
      ${namedLimitQuery}`;
  }

  // no account filter, only need to query transaction table
  return transactionOnlyQuery;
};

/**
 * Transactions queries are organized as follows: First there's an inner query that selects the
 * required number of unique transactions (identified by consensus_timestamp). And then queries other tables to
 * extract all relevant information for those transactions.
 * This function forms the inner query base based on all the query criteria specified in the REST URL
 * It selects a list of unique transactions (consensus_timestamps).
 * Also see: getTransactionsOuterQuery function
 *
 * @param {String} accountQuery SQL query that filters based on the account ids
 * @param {String} tsQuery SQL query that filters based on the timestamps for transaction table
 * @param {String} resultTypeQuery SQL query that filters based on the result types
 * @param {String} limitQuery SQL query that limits the number of unique transactions returned
 * @param {String} creditDebitQuery SQL query that filters for credit/debit transactions
 * @param {String} transactionTypeQuery SQL query that filters by transaction type
 * @param {String} order Sorting order
 * @return {String} innerQuery SQL query that filters transactions based on various types of queries
 */
const getTransactionsInnerQuery = function (
  accountQuery,
  tsQuery,
  resultTypeQuery,
  limitQuery,
  creditDebitQuery,
  transactionTypeQuery,
  order
) {
  // convert the mysql style '?' placeholder to the named parameter format, later the same named parameter is converted
  // to the same positional index, thus the caller only has to pass the value once for the same column
  const namedAccountQuery = convertToNamedQuery(accountQuery, 'acct');
  const namedTsQuery = convertToNamedQuery(tsQuery, 'ts');
  const namedLimitQuery = convertToNamedQuery(limitQuery, 'limit');

  if (creditDebitQuery) {
    // limit the query to transactions with cryptotransfer list
    return getCryptoTransferTransactionsInnerQuery(
      namedAccountQuery,
      namedTsQuery,
      resultTypeQuery,
      namedLimitQuery,
      creditDebitQuery,
      transactionTypeQuery,
      order
    );
  }

  return getGeneralTransactionsInnerQuery(
    namedAccountQuery,
    namedTsQuery,
    resultTypeQuery,
    namedLimitQuery,
    transactionTypeQuery,
    order
  );
};

const reqToSql = function (req) {
  // Parse the filter parameters for credit/debit, account-numbers, timestamp, and pagination (limit)
  const parsedQueryParams = req.query;
  const [creditDebitQuery] = utils.parseCreditDebitParams(parsedQueryParams, 'ctl.amount');
  const [accountQuery, accountParams] = utils.parseAccountIdQueryParam(parsedQueryParams, 'ctl.entity_id');
  const [tsQuery, tsParams] = utils.parseTimestampQueryParam(parsedQueryParams, 't.consensus_ns');
  const resultTypeQuery = utils.parseResultParams(req, 't.result');
  const transactionTypeQuery = utils.getTransactionTypeQuery(parsedQueryParams);
  const {query, params, order, limit} = utils.parseLimitAndOrderParams(req);
  const sqlParams = accountParams.concat(tsParams).concat(params);

  const innerQuery = getTransactionsInnerQuery(
    accountQuery,
    tsQuery,
    resultTypeQuery,
    query,
    creditDebitQuery,
    transactionTypeQuery,
    order
  );
  const sqlQuery = getTransactionsOuterQuery(innerQuery, order);

  return {
    limit,
    query: utils.convertMySqlStyleQueryToPostgres(sqlQuery),
    order,
    params: sqlParams,
  };
};

/**
 * Handler function for /transactions API.
 * @param {Request} req HTTP request object
 * @return {Promise} Promise for PostgreSQL query
 */
const getTransactions = async (req, res) => {
  // Validate query parameters first
  utils.validateReq(req);

  const query = reqToSql(req);
  if (logger.isTraceEnabled()) {
    logger.trace(`getTransactions query: ${query.query} ${JSON.stringify(query.params)}`);
  }

  // Execute query
  return pool
    .query(query.query, query.params)
    .catch((err) => {
      throw new DbError(err.message);
    })
    .then((results) => {
      const transferList = createTransferLists(results.rows);
      const ret = {
        transactions: transferList.transactions,
      };

      ret.links = {
        next: utils.getPaginationLink(
          req,
          ret.transactions.length !== query.limit,
          'timestamp',
          transferList.anchorSecNs,
          query.order
        ),
      };

      if (process.env.NODE_ENV === 'test') {
        ret.sqlQuery = results.sqlQuery;
      }

      logger.debug(`getTransactions returning ${ret.transactions.length} entries`);
      res.locals[constants.responseDataLabel] = ret;
    });
};

/**
 * Handler function for /transactions/:transaction_id API.
 * @param {Request} req HTTP request object
 * @return {} None.
 */
const getOneTransaction = async (req, res) => {
  const transactionId = TransactionId.fromString(req.params.id);
  const sqlParams = [transactionId.getEntityId().getEncodedId(), transactionId.getValidStartNs()];

  // In case of duplicate transactions, only the first succeeds
  const sqlQuery = `
    ${getSelectClauseWithTokenTransferOrder()}
    FROM transaction t
    JOIN t_transaction_results ttr ON ttr.proto_id = t.result
    JOIN t_transaction_types ttt ON ttt.proto_id = t.type
    LEFT JOIN crypto_transfer ctl ON ctl.consensus_timestamp = t.consensus_ns
    LEFT JOIN token_transfer ttl
      ON t.type = ${constants.transactionTypes.CRYPTOTRANSFER}
      AND t.consensus_ns = ttl.consensus_timestamp
    WHERE t.payer_account_id = ?
       AND  t.valid_start_ns = ?
    GROUP BY consensus_ns, ctl_entity_id, ctl.amount, ttr.result, ttt.name, t.payer_account_id, t.memo,
      t.valid_start_ns, t.node_account_id, t.charged_tx_fee, t.valid_duration_seconds, t.max_fee, t.transaction_hash
    ORDER BY consensus_ns ASC, ctl_entity_id ASC, ctl.amount ASC`;

  const pgSqlQuery = utils.convertMySqlStyleQueryToPostgres(sqlQuery);
  if (logger.isTraceEnabled()) {
    logger.trace(`getOneTransaction query: ${pgSqlQuery} ${JSON.stringify(sqlParams)}`);
  }

  // Execute query
  return pool
    .query(pgSqlQuery, sqlParams)
    .catch((err) => {
      throw new DbError(err.message);
    })
    .then((results) => {
      const transferList = createTransferLists(results.rows);
      if (transferList.transactions.length === 0) {
        throw new NotFoundError('Not found');
      }
      logger.debug(`getOneTransaction returning ${transferList.transactions.length} entries`);
      res.locals[constants.responseDataLabel] = {
        transactions: transferList.transactions,
      };
    });
};
module.exports = {
  getTransactions,
  getOneTransaction,
  createTransferLists,
  reqToSql,
  buildWhereClause,
  getTransactionsInnerQuery,
  getTransactionsOuterQuery,
};
