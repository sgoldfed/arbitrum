/*
 * Copyright 2019-2020, Offchain Labs, Inc.
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

#include <avm/machinestate/datastack.hpp>

#include <avm_values/util.hpp>
#include <bigint_utils.hpp>

uint256_t Datastack::hash() const {
    auto h_value = getHashPreImage();
    return h_value.hash();
}

uint256_t Datastack::getTotalValueSize() const {
    auto h_value = getHashPreImage();
    return h_value.getSize();
}

HashPreImage Datastack::getHashPreImage() const {
    if (values.empty()) {
        return Tuple().getHashPreImage();
    } else {
        calculateAllHashes();
        return hashes.back();
    }
}

std::pair<HashPreImage, std::vector<unsigned char>> Datastack::marshalForProof(
    const std::vector<bool>& stackInfo,
    const Code& code) {
    calculateAllHashes();
    Datastack c = *this;
    std::vector<unsigned char> buf;
    for (auto const& si : stackInfo) {
        value val = c.pop();
        if (si) {
            ::marshalForProof(val, buf, code);
        } else {
            marshalStub(val, buf, code);
        }
    }
    return std::make_pair(c.getHashPreImage(), std::move(buf));
}

std::ostream& operator<<(std::ostream& os, const Datastack& val) {
    os << "[";
    for (uint64_t i = 0; i < val.values.size(); i++) {
        os << val.values[val.values.size() - 1 - i];
        if (i < val.values.size() - 1) {
            os << ", ";
        }
    }
    os << "]";
    return os;
}

Tuple Datastack::getTupleRepresentation(TuplePool* pool) const {
    Tuple rep;
    for (size_t i = 0; i < values.size(); i++) {
        rep = Tuple(values[values.size() - 1 - i], rep, pool);
    }
    return rep;
}

Datastack::Datastack(Tuple tuple_rep) : Datastack() {
    Tuple ret = tuple_rep;
    while (ret.tuple_size() == 2) {
        push(ret.get_element(0));
        ret = nonstd::get<Tuple>(ret.get_element(1));
    }
}

void Datastack::addHash() const {
    HashPreImage prev;
    if (hashes.size() > 0) {
        prev = hashes.back();
    } else {
        prev = Tuple().getHashPreImage();
    }

    auto newVal = values[hashes.size()];
    TuplePool pool;
    auto tup = Tuple(newVal, prev, &pool);
    hashes.emplace_back(tup.getHashPreImage());
}

void Datastack::calculateAllHashes() const {
    while (hashes.size() < values.size()) {
        addHash();
    }
}
