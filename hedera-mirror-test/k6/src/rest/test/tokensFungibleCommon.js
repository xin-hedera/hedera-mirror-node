// SPDX-License-Identifier: Apache-2.0

import http from 'k6/http';

import {isValidListResponse, RestTestScenarioBuilder} from '../libex/common.js';
import {tokenListName} from '../libex/constants.js';

const urlTag = '/tokens?type=FUNGIBLE_COMMON';

const {options, run, setup} = new RestTestScenarioBuilder()
  .name('tokensFungibleCommon') // use unique scenario name among all tests
  .tags({url: urlTag})
  .request((testParameters) => {
    const url = `${testParameters['BASE_URL_PREFIX']}${urlTag}`;
    return http.get(url);
  })
  .check('Tokens FUNGIBLE_COMMON OK', (r) => isValidListResponse(r, tokenListName))
  .build();

export {options, run, setup};
