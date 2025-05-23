// SPDX-License-Identifier: Apache-2.0

package com.hedera.mirror.web3.repository;

import static com.hedera.mirror.web3.evm.config.EvmConfiguration.CACHE_MANAGER_CONTRACT;
import static com.hedera.mirror.web3.evm.config.EvmConfiguration.CACHE_NAME_CONTRACT;

import com.hedera.mirror.common.domain.contract.Contract;
import java.util.Optional;
import org.springframework.cache.annotation.Cacheable;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.CrudRepository;

public interface ContractRepository extends CrudRepository<Contract, Long> {

    @Cacheable(cacheNames = CACHE_NAME_CONTRACT, cacheManager = CACHE_MANAGER_CONTRACT, unless = "#result == null")
    @Query(value = "select runtime_bytecode from contract where id = :contractId", nativeQuery = true)
    Optional<byte[]> findRuntimeBytecode(final Long contractId);
}
