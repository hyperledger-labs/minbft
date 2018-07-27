// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Sergey Fedorov <sergey.fedorov@neclab.eu>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#undef NDEBUG // make sure `assert()` is not an empty macro
#include <assert.h>
#include <stdlib.h>
#include <stdbool.h>
#include <string.h>
#include <stdio.h>

#include "usig.h"

const char *enclave_file;

// NOTE: We only do *limited* testing here. For instance, we don't
// check correctness of the signature (certificate) produced by the
// enclave because it's cumbersome to do so in C. That's easier to
// test in Golang, together with the certificate verification
// implementation.

static void test_init_destroy()
{
        sgx_enclave_id_t eid;

        assert(usig_init(enclave_file, &eid, NULL, 0) == SGX_SUCCESS);
        assert(usig_destroy(eid) == SGX_SUCCESS);
}

static inline bool signature_is_equal(usig_ui *ui1, usig_ui *ui2)
{
        return memcmp(&ui1->signature, &ui2->signature, sizeof(ui1->signature)) == 0;
}

static void test_seal_key()
{
        sgx_enclave_id_t usig;
        void *sealed_data;
        size_t sealed_data_size;

        assert(usig_init(enclave_file, &usig, NULL, 0) == SGX_SUCCESS);
        assert(usig_seal_key(usig, &sealed_data,
                             &sealed_data_size) == SGX_SUCCESS);
        assert(usig_destroy(usig) == SGX_SUCCESS);
        assert(usig_init(enclave_file, &usig, sealed_data,
                         sealed_data_size) == SGX_SUCCESS);
        free(sealed_data);
        assert(usig_destroy(usig) == SGX_SUCCESS);
}

static void test_create_ui()
{
        sgx_enclave_id_t usig;
        void *sealed_data;
        size_t sealed_data_size;
        usig_ui ui1, ui2, ui3;
        sgx_sha256_hash_t digest = "TEST DIGEST";

        assert(usig_init(enclave_file, &usig, NULL, 0) == SGX_SUCCESS);
        assert(usig_seal_key(usig, &sealed_data,
                             &sealed_data_size) == SGX_SUCCESS);
        assert(usig_create_ui(usig, digest, &ui1) == SGX_SUCCESS);
        // The first counter value must be zero
        assert(ui1.counter == 1);
        assert(usig_create_ui(usig, digest, &ui2) == SGX_SUCCESS);
        // The counter must be monotonic and sequential
        assert(ui2.counter == ui1.counter + 1);
        // Certificate must be unique for each counter value
        assert(!signature_is_equal(&ui1, &ui2));
        // Epoch should be the same for the same USIG instance
        assert(ui1.epoch == ui2.epoch);
        assert(usig_destroy(usig) == SGX_SUCCESS);
        // Recreate USIG using the sealed secret from the first instance
        assert(usig_init(enclave_file, &usig, sealed_data,
                         sealed_data_size) == SGX_SUCCESS);
        assert(usig_create_ui(usig, digest, &ui3) == SGX_SUCCESS);
        // Must fetch a fresh counter value
        assert(ui3.counter == 1);
#ifndef SGX_SIM_MODE
        // Apparently, SGX SDK in the simulation mode uses current
        // time in *seconds* to seed random number generation. We
        // don't want to wait that long and skip these checks.
        // Check for uniqueness of the epoch and certificate produced
        // by the new instance of the enclave
        assert(ui1.epoch != ui3.epoch);
        assert(!signature_is_equal(&ui1, &ui3));
#endif
        assert(usig_destroy(usig) == SGX_SUCCESS);
        free(sealed_data);
}

int main(int argc, const char **argv)
{
        assert(argc == 2);
        enclave_file = argv[1];

        test_init_destroy();
        test_seal_key();
        test_create_ui();

        puts("PASS");
}
