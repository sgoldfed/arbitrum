/*
 * Copyright 2019, Offchain Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#include <data_storage/blockstore.hpp>
#include <data_storage/confirmednodestore.hpp>
#include <data_storage/datastorage.hpp>
#include <data_storage/storageresult.hpp>
#include <string>

#include <rocksdb/options.h>
#include <avm_values/codepoint.hpp>
#include <avm_values/tuple.hpp>

#include <rocksdb/utilities/transaction.h>
#include <rocksdb/utilities/transaction_db.h>

DataStorage::DataStorage(const std::string& db_path) {
    rocksdb::TransactionDBOptions txn_options;
    rocksdb::Options options;
    options.create_if_missing = true;
    options.create_missing_column_families = true;

    txn_db_path = std::move(db_path);

    std::vector<rocksdb::ColumnFamilyDescriptor> column_families;
    column_families.push_back(rocksdb::ColumnFamilyDescriptor(
        rocksdb::kDefaultColumnFamilyName, rocksdb::ColumnFamilyOptions()));
    column_families.push_back(rocksdb::ColumnFamilyDescriptor(
        "blocks", rocksdb::ColumnFamilyOptions()));
    column_families.push_back(rocksdb::ColumnFamilyDescriptor(
        "nodes", rocksdb::ColumnFamilyOptions()));

    rocksdb::TransactionDB* db = nullptr;
    std::vector<rocksdb::ColumnFamilyHandle*> handles;
    auto status = rocksdb::TransactionDB::Open(
        options, txn_options, txn_db_path, column_families, &handles, &db);

    if (!status.ok()) {
        std::cerr << "rocksdb construction status: " << status.ToString()
                  << std::endl;

        throw std::exception();
    }
    assert(status.ok());
    txn_db = std::unique_ptr<rocksdb::TransactionDB>(db);
    default_column = std::unique_ptr<rocksdb::ColumnFamilyHandle>(handles[0]);
    blocks_column = std::unique_ptr<rocksdb::ColumnFamilyHandle>(handles[1]);
    node_column = std::unique_ptr<rocksdb::ColumnFamilyHandle>(handles[2]);
}

rocksdb::Status DataStorage::closeDb() {
    blocks_column.reset();
    default_column.reset();
    node_column.reset();
    auto s = txn_db->Close();
    txn_db.reset();
    return s;
}
