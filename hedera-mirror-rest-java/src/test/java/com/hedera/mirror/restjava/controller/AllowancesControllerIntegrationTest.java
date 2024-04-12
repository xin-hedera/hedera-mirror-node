/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
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
 */

package com.hedera.mirror.restjava.controller;

import com.hedera.mirror.common.domain.DomainBuilder;
import com.hedera.mirror.common.domain.entity.EntityId;
import com.hedera.mirror.common.domain.entity.NftAllowance;
import com.hedera.mirror.common.util.DomainUtils;
import com.hedera.mirror.rest.model.Error;
import com.hedera.mirror.rest.model.ErrorStatusMessagesInner;
import com.hedera.mirror.rest.model.NftAllowancesResponse;
import com.hedera.mirror.restjava.RestJavaIntegrationTest;
import com.hedera.mirror.restjava.mapper.NftAllowanceMapper;
import jakarta.annotation.Resource;
import org.apache.commons.codec.binary.Base32;
import org.assertj.core.api.InstanceOfAssertFactories;
import org.assertj.core.api.ThrowableAssert;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.junit.jupiter.params.provider.ValueSource;
import org.junit.runner.RunWith;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.server.LocalServerPort;
import org.springframework.test.context.junit4.SpringRunner;
import org.springframework.web.client.HttpClientErrorException;
import org.springframework.web.client.RestClient;

import java.util.Collection;
import java.util.Collections;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.assertj.core.api.AssertionsForClassTypes.assertThat;

@RunWith(SpringRunner.class)
@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
class AllowancesControllerIntegrationTest extends RestJavaIntegrationTest {

    @LocalServerPort
    private int port;

    private String callUri;

    @Resource
    private DomainBuilder domainBuilder;

    @Resource
    private NftAllowanceMapper mapper;

    private RestClient restClient;

    @BeforeEach
    void setup() {
        callUri = "http://localhost:%d/api/v1/accounts/{id}/allowances/nfts".formatted(port);
        restClient = RestClient.builder()
                .baseUrl(callUri)
                .defaultHeader("Accept", "application/json")
                .defaultHeader("Access-Control-Request-Method", "GET")
                .defaultHeader("Origin", "http://example.com")
                .build();
    }

    @Test
    void nftAllowancesEntityId() {
        var entity = domainBuilder.entity().persist();
        var allowance1 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(entity.getId()))
                .persist();
        var allowance2 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(allowance1.getOwner()))
                .persist();
        Collection<NftAllowance> collection = List.of(allowance1, allowance2);

        var result = restClient
                .get()
                .uri("", allowance1.getOwner())
                .retrieve()
                .body(NftAllowancesResponse.class);
        assertThat(result.getAllowances()).isEqualTo(mapper.map(collection));
        assertThat(result.getLinks().getNext()).isNull();
    }

    @Test
    void nftAllowancesEvmAddress() {
        var entity = domainBuilder.entity().persist();
        var allowance1 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(entity.getId()))
                .persist();
        var allowance2 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(allowance1.getOwner()))
                .persist();
        Collection<NftAllowance> collection = List.of(allowance1, allowance2);

        var result = restClient
                .get()
                .uri("", DomainUtils.bytesToHex(entity.getEvmAddress()))
                .retrieve()
                .body(NftAllowancesResponse.class);
        assertThat(result.getAllowances()).isEqualTo(mapper.map(collection));
        assertThat(result.getLinks().getNext()).isNull();
    }

    @Test
    void nftAllowancesAlias() {
        var entity = domainBuilder.entity().persist();
        var allowance1 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(entity.getId()))
                .persist();
        var allowance2 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(allowance1.getOwner()))
                .persist();
        Collection<NftAllowance> collection = List.of(allowance1, allowance2);

        var result = restClient
                .get()
                .uri("", new Base32().encodeAsString(entity.getAlias()))
                .retrieve()
                .body(NftAllowancesResponse.class);
        assertThat(result.getAllowances()).isEqualTo(mapper.map(collection));
        assertThat(result.getLinks().getNext()).isNull();
    }

    @Test
    void nftAllowancesOrderAsc() {
        var entity = domainBuilder.entity().persist();

        // Creating nft allowances
        var allowance1 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(entity.getId()))
                .persist();
        domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(allowance1.getOwner()))
                .persist();

        // Setting up the url params
        var uriParams = "?account.id=gte:0.0.1000&owner=true&token.id=gt:0.0.1000&limit=1&order=asc";

        var nextLink =
                "/api/v1/accounts/%s/allowances/nfts?account.id=gte:%s&owner=true&token.id=gt:%s&limit=1&order=asc"
                        .formatted(
                                allowance1.getOwner(),
                                EntityId.of(allowance1.getSpender()),
                                EntityId.of(allowance1.getTokenId()));

        // Performing the GET operation
        var result = restClient
                .get()
                .uri(uriParams, allowance1.getOwner())
                .retrieve()
                .body(NftAllowancesResponse.class);

        assertThat(result.getAllowances()).isEqualTo(mapper.map(List.of(allowance1)));
        assertThat(result.getLinks().getNext()).isEqualTo(nextLink);
    }

    @Test
    void nftAllowancesNoOperators() {
        var entity = domainBuilder.entity().persist();
        // Creating nft allowances
        var allowance1 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(entity.getId()))
                .persist();
        var allowance2 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(allowance1.getOwner()))
                .persist();

        var uriParams = "?account.id={account.id}&limit=1&order=asc";
        var nextLink = "/api/v1/accounts/%s/allowances/nfts?account.id=gte:%s&limit=1&order=asc&token.id=gt:%s"
                .formatted(
                        allowance1.getOwner(),
                        EntityId.of(allowance2.getSpender()),
                        EntityId.of(allowance2.getTokenId()));

        // Performing the GET operation
        var result = restClient
                .get()
                .uri(uriParams, allowance1.getOwner(), EntityId.of(allowance1.getSpender()))
                .retrieve()
                .body(NftAllowancesResponse.class);

        // This test will need to change after the new repository layer is integrated to return the correct result for
        // spenderId = allowance.spender()
        assertThat(result.getAllowances()).isEqualTo(mapper.map(List.of(allowance2)));
        assertThat(result.getLinks().getNext()).isEqualTo(nextLink);
    }

    @Test
    void nftAllowancesOrderDesc() {
        var entity = domainBuilder.entity().persist();
        // Creating nft allowances
        var allowance1 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(entity.getId()))
                .persist();
        var allowance2 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(allowance1.getOwner()))
                .persist();

        var uriParams = "?account.id=gte:0.0.1000&owner=true&token.id=gt:0.0.1000&limit=1&order=desc";
        var nextLink =
                "/api/v1/accounts/%s/allowances/nfts?account.id=lte:%s&owner=true&token.id=lt:%s&limit=1&order=desc"
                        .formatted(
                                allowance2.getOwner(),
                                EntityId.of(allowance2.getSpender()),
                                EntityId.of(allowance2.getTokenId()));

        // Performing the GET operation
        var result = restClient
                .get()
                .uri(uriParams, allowance1.getOwner())
                .retrieve()
                .body(NftAllowancesResponse.class);

        assertThat(result.getAllowances()).isEqualTo(mapper.map(List.of(allowance2)));
        assertThat(result.getLinks().getNext()).isEqualTo(nextLink);
    }

    @Test
    void nftAllowancesOwnerFalse() {
        var entity = domainBuilder.entity().persist();
        // Creating nft allowances
        var allowance1 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.spender(entity.getId()))
                .persist();
        domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.spender(allowance1.getSpender()))
                .persist();

        var uriParams = "?account.id=gte:0.0.1000&owner=false&token.id=gt:0.0.1000&limit=1&order=asc";
        var nextLink =
                "/api/v1/accounts/%s/allowances/nfts?account.id=gte:%s&owner=false&token.id=gt:%s&limit=1&order=asc"
                        .formatted(
                                allowance1.getSpender(),
                                EntityId.of(allowance1.getOwner()),
                                EntityId.of(allowance1.getTokenId()));

        // Performing the GET operation
        var result = restClient
                .get()
                .uri(uriParams, allowance1.getSpender())
                .retrieve()
                .body(NftAllowancesResponse.class);

        assertThat(result.getAllowances()).isEqualTo(mapper.map(List.of(allowance1)));
        assertThat(result.getLinks().getNext()).isEqualTo(nextLink);
    }

    @Test
    void nftAllowancesEmptyNextLink() {
        var entity = domainBuilder.entity().persist();

        // Creating nft allowances
        var allowance1 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.spender(entity.getId()))
                .persist();
        domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(allowance1.getOwner()))
                .persist();

        var uriParams = "?account.id=gte:0.0.5000&owner=true&token.id=gt:0.0.5000&limit=1&order=asc";

        // Performing the GET operation
        var result = restClient
                .get()
                .uri(uriParams, allowance1.getSpender())
                .retrieve()
                .body(NftAllowancesResponse.class);

        assertThat(result.getAllowances()).isEqualTo(Collections.EMPTY_LIST);
        assertThat(result.getLinks().getNext()).isNull();
    }

    @ParameterizedTest
    @CsvSource({
        "0.0.1001,1.2.3.4,0.0.2000,false,2,asc",
        "0.0.1001,0.0.2000,1.2.3.4,false,2,asc",
        "0.0.1001,0.0.3000,0.0.2000,false,-1,asc",
        "0.0.1001,0.0.3000,0.0.2000,false,111,asc",
        "0.0.1001,0.0.3000,0.0.2000,false,3,ttt",
        "0.0.1001,gee:0.0.3000,0.0.2000,false,3,asc",
        "null,gte:0.0.3000,0.0.2000,false,3,asc",
        "0.0.4294967296,gt:0.0.3000,gte:0.0.3000,false,3,asc",
        "9223372036854775807,0.0.3000,0.0.2000,false,3,asc",
        "0x00000001000000000000000200000000000000034,0.0.3000,0.0.2000,false,3,asc"
    })
    void nftAllowancesInvalidParams(String id, String accountId, String tokenId, String owner, String limit, String order) {

        String uriParams = "?account.id={accountId}&owner={owner}&token.id={token.id}&limit={limit}&order={order}";

        // Performing the GET operation
        assertThatThrownBy(() -> restClient
                .get()
                .uri(uriParams, id, accountId, owner, tokenId, limit, order)
                .retrieve()
                .body(NftAllowancesResponse.class)
        )
                .isInstanceOf(HttpClientErrorException.BadRequest.class)
                .hasMessageContaining("Bad Request");
    }

    @Test
    void nftAllowancesUnsupportedMediaException() {

        // Performing the POST operation
        assertThatThrownBy(() -> restClient
                .post()
                .uri(callUri, "0.0.1000")
                .retrieve()
                .body(NftAllowancesResponse.class)
        )
                .isInstanceOf(HttpClientErrorException.MethodNotAllowed.class)
                .hasMessageContaining("Method Not Allowed");
    }

    @ParameterizedTest
    @ValueSource(
            strings = {
                    "0.0.1000",
                    "0.1000",
                    "1000",
                    "0.0x000000000000000000000000000000000186Fb1b",
                    "0.0.0x000000000000000000000000000000000186Fb1b",
                    "0x000000000000000000000000000000000186Fb1b",
                    "0.0.AABBCC22",
                    "0.AABBCC22",
                    "AABBCC22"
            })
    void nftAllowancesNotFoundException(String accountId) {
        validateError(() -> restClient.get()
                        .uri(callUri, accountId)
                        .retrieve()
                        .body(NftAllowancesResponse.class)
                , HttpClientErrorException.NotFound.class
                ,  "No account found for the given ID"
                , "Not Found");
    }

    @Test
    void nftAllowancesTokenIdValidation() {

        String uriParams = "?owner=true&token.id=gt:0.0.1000&limit=1&order=asc";
        var entity = domainBuilder.entity().persist();

        // Creating nft allowances
        var allowance1 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(entity.getId()))
                .persist();

        // Performing the GET operation
        validateError(() -> restClient.get()
                        .uri(uriParams, allowance1.getOwner())
                        .retrieve()
                        .body(NftAllowancesResponse.class)
                , HttpClientErrorException.BadRequest.class
                , "token.id parameter must have account.id present"
                , "Bad Request");
    }

    @Test
    void nftAllowancesEntityIdRangeOperatorValidation() {

        String uriParams = "?owner=true&account.id=gte:0.0.1000&token.id=ne:0.0.1000&limit=1&order=asc";
        var entity = domainBuilder.entity().persist();

        // Creating nft allowances
        var allowance1 = domainBuilder
                .nftAllowance()
                .customize(nfta -> nfta.owner(entity.getId()))
                .persist();

        // Performing the GET operation
        validateError(() -> restClient.get()
                        .uri(uriParams, allowance1.getOwner())
                        .retrieve()
                        .body(NftAllowancesResponse.class)
       , HttpClientErrorException.BadRequest.class
                , "Invalid range operator ne. This operator is not supported"
                , "Bad Request");
    }


    private void validateError(ThrowableAssert.ThrowingCallable callable, Class<? extends HttpClientErrorException> clazz, String message, String description) {
        assertThatThrownBy(callable).isInstanceOf(clazz)
                .hasMessageContaining(description)
                .asInstanceOf(InstanceOfAssertFactories.type(clazz))
                .extracting(r -> r.getResponseBodyAs(Error.class).getStatus().getMessages().get(0))
                .returns(null, ErrorStatusMessagesInner::getData)
                .returns(null, ErrorStatusMessagesInner::getDetail)
                .returns(message, ErrorStatusMessagesInner::getMessage);
    }

}