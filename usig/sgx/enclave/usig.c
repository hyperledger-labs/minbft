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

#include <stdbool.h>
#include <string.h>

#include <sgx_trts.h>
#include <sgx_tseal.h>

#include "usig_t.h"

static sgx_ec256_private_t usig_priv_key;
static sgx_ec256_public_t usig_pub_key;
static uint64_t usig_epoch;
static bool initialized;

typedef struct __attribute__((__packed__)) {
    sgx_sha256_hash_t digest;
    uint64_t epoch;
    uint64_t counter;
} usig_cert_data_t;

sgx_status_t ecall_usig_create_ui(sgx_sha256_hash_t digest,
                                  uint64_t *counter,
                                  sgx_ec256_signature_t *signature)
{
        static uint64_t usig_counter = 1;
        sgx_status_t ret;
        sgx_ecc_state_handle_t ecc_handle;
        sgx_ec256_signature_t signature_buf;
        usig_cert_data_t data;

        if (!initialized) {
                ret = SGX_ERROR_UNEXPECTED;
                goto out;
        }

        ret = sgx_ecc256_open_context(&ecc_handle);
        if (ret != SGX_SUCCESS) {
                goto out;
        }

        memcpy(data.digest, digest, sizeof(data.digest));
        data.epoch = usig_epoch;
        *counter = data.counter = usig_counter;

        ret = sgx_ecdsa_sign((uint8_t *)&data, sizeof(data),
                             &usig_priv_key, &signature_buf, ecc_handle);
        if (ret != SGX_SUCCESS) {
                goto close_context;
        }

        // Increment the internal counter just before going to expose
        // a valid signature to the untrusted world. That makes sure
        // the counter value cannot be reused to sign another message.
        usig_counter++;
        memcpy(signature, &signature_buf, sizeof(signature_buf));

close_context:
        sgx_ecc256_close_context(ecc_handle);
out:
        return ret;
}

sgx_status_t ecall_usig_get_epoch(uint64_t *epoch)
{
        if (!initialized) {
                return SGX_ERROR_UNEXPECTED;
        }

        *epoch = usig_epoch;

        return SGX_SUCCESS;
}

sgx_status_t ecall_usig_get_pub_key(sgx_ec256_public_t *pub_key)
{
        if (!initialized) {
                return SGX_ERROR_UNEXPECTED;
        }

        memcpy(pub_key, &usig_pub_key, sizeof(usig_pub_key));

        return SGX_SUCCESS;
}

sgx_status_t ecall_usig_get_sealed_key_size(uint32_t *size)
{
        *size = sgx_calc_sealed_data_size(sizeof(usig_pub_key), sizeof(usig_priv_key));

        return SGX_SUCCESS;
}

sgx_status_t ecall_usig_seal_key(void *sealed_data, uint32_t sealed_data_size)
{
        if (!initialized) {
                return SGX_ERROR_UNEXPECTED;
        }

        return sgx_seal_data(sizeof(usig_pub_key), (void *)&usig_pub_key,
                            sizeof(usig_priv_key), (void *)&usig_priv_key,
                            sealed_data_size, sealed_data);
}

static sgx_status_t generate_key(void)
{
        sgx_status_t ret;
        sgx_ecc_state_handle_t ecc_handle;

        ret = sgx_ecc256_open_context(&ecc_handle);
        if (ret != SGX_SUCCESS) {
                goto out;
        }

        // Create an key pair to produce USIG certificates.
        ret = sgx_ecc256_create_key_pair(&usig_priv_key, &usig_pub_key, ecc_handle);
        if (ret != SGX_SUCCESS) {
                goto close_context;
        }

close_context:
        sgx_ecc256_close_context(ecc_handle);
out:
        return ret;
}

static sgx_status_t unseal_key(void *data, uint32_t size)
{
        sgx_status_t ret;
        uint32_t pub_key_size = sizeof(usig_pub_key);
        uint32_t priv_key_size = sizeof(usig_priv_key);

        if (size != sgx_calc_sealed_data_size(pub_key_size, priv_key_size)) {
                ret = SGX_ERROR_UNEXPECTED;
                goto out;
        }

        ret = sgx_unseal_data((sgx_sealed_data_t *)data,
                               (void *)&usig_pub_key, &pub_key_size,
                               (void *)&usig_priv_key, &priv_key_size);
        if (ret != SGX_SUCCESS) {
                goto out;
        }

        if (pub_key_size != sizeof(usig_pub_key) ||
            priv_key_size != sizeof(usig_priv_key)) {
                ret =  SGX_ERROR_UNEXPECTED;
                goto out;
        }

out:
        return ret;
}

sgx_status_t ecall_usig_init(void *sealed_data, uint32_t sealed_data_size)
{
        sgx_status_t ret;

        if (initialized) {
                ret = SGX_ERROR_UNEXPECTED;
                goto out;
        }

        // Create a random epoch value. Each instance of USIG should
        // have a unique epoch value to be able to guarantee unique,
        // sequential and monotonic counter values given an epoch
        // value.
        ret = sgx_read_rand((void *)&usig_epoch, sizeof(usig_epoch));
        if (ret != SGX_SUCCESS) {
                goto out;
        }

        ret = sealed_data != NULL ?
                unseal_key(sealed_data, sealed_data_size) :
                generate_key();
        if (ret != SGX_SUCCESS) {
                goto out;
        }

        initialized = true;

out:
        return ret;
}
