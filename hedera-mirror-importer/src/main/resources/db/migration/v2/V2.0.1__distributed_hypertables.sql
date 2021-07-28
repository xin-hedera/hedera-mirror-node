-------------------
-- Create hyper tables for tables that have mostly insert logic
-- Set chunk_time_interval using parameterized value, usually default of 604800000000000 ns (7 days)
-- Set create_default_indexes to false for tables where a primary key is needed or an index in ASC order is needed.
-- By default TimescaleDB adds an index in DESC order for partitioning column
-------------------

-- assessed_custom_fee
select create_distributed_hypertable('assessed_custom_fee', 'consensus_timestamp',
    partitioning_column => 'consensus_timestamp', chunk_time_interval => ${chunkTimeInterval},
    create_default_indexes => false, if_not_exists => true);

-- account_balance
select create_distributed_hypertable('account_balance', 'consensus_timestamp',
    partitioning_column => 'consensus_timestamp', chunk_time_interval => ${chunkTimeInterval},
    create_default_indexes => false, if_not_exists => true);

-- account_balance_file
select create_distributed_hypertable('account_balance_file', 'consensus_timestamp',
    partitioning_column => 'consensus_timestamp', chunk_time_interval => ${chunkTimeInterval},
    create_default_indexes => false, if_not_exists => true);

-- address_book hyper table creation skipped as small tables won't benefit from timescaledb scalability

-- address_book_entry hyper table creation skipped as small tables won't benefit from timescaledb scalability

-- address_book_service_endpoint hyper table creation skipped as small tables won't benefit from timescaledb scalability

-- contract_result
select create_distributed_hypertable('contract_result', 'consensus_timestamp',
    partitioning_column => 'consensus_timestamp', chunk_time_interval => ${chunkTimeInterval},
    create_default_indexes => false, if_not_exists => true);

-- crypto_transfer
select create_distributed_hypertable('crypto_transfer', 'consensus_timestamp',
    partitioning_column => 'consensus_timestamp', chunk_time_interval => ${chunkTimeInterval},
    create_default_indexes => false, if_not_exists => true);

-- custom_fee
select create_distributed_hypertable('custom_fee', 'created_timestamp', partitioning_column => 'consensus_timestamp',
    chunk_time_interval => ${chunkTimeInterval}, create_default_indexes => false, if_not_exists => true);

-- event_file
select create_distributed_hypertable('event_file', 'consensus_end', partitioning_column => 'consensus_end',
    chunk_time_interval => ${chunkTimeInterval}, create_default_indexes => false, if_not_exists => true);

-- file_data
select create_distributed_hypertable('file_data', 'consensus_timestamp', partitioning_column => 'consensus_timestamp',
    chunk_time_interval => ${chunkTimeInterval}, create_default_indexes => false, if_not_exists => true);

-- live_hash
select create_distributed_hypertable('live_hash', 'consensus_timestamp', partitioning_column => 'consensus_timestamp',
    chunk_time_interval => ${chunkTimeInterval}, create_default_indexes => false,
    if_not_exists => true);

-- nft
select create_hypertable('nft', 'created_timestamp', chunk_time_interval => ${chunkTimeInterval},
    create_default_indexes => false, if_not_exists => true);

-- nft_transfer
select create_distributed_hypertable('nft_transfer', 'consensus_timestamp',
    partitioning_column => 'consensus_timestamp', chunk_time_interval => ${chunkTimeInterval},
    create_default_indexes => false, if_not_exists => true);

-- non_fee_transfer
select create_distributed_hypertable('non_fee_transfer', 'consensus_timestamp',
    partitioning_column => 'consensus_timestamp', chunk_time_interval => ${chunkTimeInterval},
    create_default_indexes => false, if_not_exists => true);

-- record_file
select create_distributed_hypertable('record_file', 'consensus_end', partitioning_column => 'consensus_end',
    chunk_time_interval => ${chunkTimeInterval}, create_default_indexes => false, if_not_exists => true);

-- schedule
select create_distributed_hypertable('schedule', 'consensus_timestamp', partitioning_column => 'consensus_timestamp',
    chunk_time_interval => ${chunkTimeInterval}, create_default_indexes => false, if_not_exists => true);

-- entity
select create_hypertable('entity', 'id', chunk_time_interval => ${chunkIdInterval},
    create_default_indexes => false, if_not_exists => true);

-- t_entity_types hyper table creation skipped as it serves only as a reference table and rarely gets updated

-- t_transaction_results hyper table creation skipped as it serves only as a reference table and rarely gets updated

-- t_transaction_types hyper table creation skipped as it serves only as a reference table and rarely gets updated

-- token
select create_hypertable('token', 'created_timestamp', chunk_time_interval => ${chunkTimeInterval},
    create_default_indexes => false, if_not_exists => true);

-- token_account
select create_distributed_hypertable('token_account', 'created_timestamp', partitioning_column => 'created_timestamp',
    chunk_time_interval => ${chunkTimeInterval}, create_default_indexes => false, if_not_exists => true);

-- token_balance
select create_distributed_hypertable('token_balance', 'consensus_timestamp',
    partitioning_column => 'consensus_timestamp', chunk_time_interval => ${chunkTimeInterval},
    create_default_indexes => false, if_not_exists => true);

-- token_transfer
select create_distributed_hypertable('token_transfer', 'consensus_timestamp',
    partitioning_column => 'consensus_timestamp', chunk_time_interval => ${chunkTimeInterval},
    create_default_indexes => false, if_not_exists => true);

-- topic_message
select create_distributed_hypertable('topic_message', 'consensus_timestamp',
    partitioning_column => 'consensus_timestamp', chunk_time_interval => ${chunkTimeInterval},
    create_default_indexes => false, if_not_exists => true);

-- transaction
select create_distributed_hypertable('transaction', 'consensus_ns', partitioning_column => 'consensus_ns',
    chunk_time_interval => ${chunkTimeInterval}, create_default_indexes => false, if_not_exists => true);

-- transaction_signature
select create_distributed_hypertable('transaction_signature', 'consensus_timestamp',
    partitioning_column => 'consensus_timestamp', chunk_time_interval => ${chunkTimeInterval},
    create_default_indexes => false, if_not_exists => true);
