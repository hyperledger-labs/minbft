/*
 * Copyright (c) 2020 NEC Solution Innovators, Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/*
 * This program is based on `sgx_enable.c` in the following repository:
 * https://github.com/intel/sgx-software-enable
 *
 * Copyright (C) 2011-2019 Intel Corporation. All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 *
 *   * Redistributions of source code must retain the above copyright
 *     notice, this list of conditions and the following disclaimer.
 *   * Redistributions in binary form must reproduce the above copyright
 *     notice, this list of conditions and the following disclaimer in
 *     the documentation and/or other materials provided with the
 *     distribution.
 *   * Neither the name of Intel Corporation nor the names of its
 *     contributors may be used to endorse or promote products derived
 *     from this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 *
 */

#include <stdio.h>
#include <sgx_capable.h>

int main (int argc, char *argv[])
{
	sgx_status_t status;
	sgx_device_status_t result;

	status= sgx_cap_get_status(&result);
	if (status != SGX_SUCCESS) {
		switch(status) {
		case SGX_ERROR_NO_PRIVILEGE:
			fprintf(stderr, "could not examine the EFI filesystem\n");
			return 1;
		case SGX_ERROR_INVALID_PARAMETER:
		case SGX_ERROR_UNEXPECTED:
			fprintf(stderr, "could not get SGX status: ");
		default:
			fprintf(stderr, "sgx_cap_get_status returned 0x%04x\n", status);
			return 1;
		}
	}

	if (result == SGX_ENABLED) {
		printf("Intel SGX is supported and enabled on this system.\n");
		return 0;
	} else if (result == SGX_DISABLED_UNSUPPORTED_CPU) {
		printf("This CPU does not support Intel SGX.\n");
		return 1;
	} else if (result == SGX_DISABLED_LEGACY_OS) {
		printf("This processor supports Intel SGX but was booted in legacy mode.\n");
		printf("A UEFI boot is required to determine whether or not your BIOS\n");
		printf("supports Intel SGX.\n");
		return 1;
	} else if (result == SGX_DISABLED) {
		printf("Intel SGX is explicitly disabled on your system. It may be\n");
		printf("disabled in the BIOS, or the BIOS may not support Intel SGX.\n");
		return 1;
	} else if (result == SGX_DISABLED_MANUAL_ENABLE) {
		printf("Intel SGX is explicitly disabled, and your BIOS does not\n");
		printf("support the \"software enable\" option. Check your BIOS for an\n");
		printf("explicit option to enable Intel SGX.\n");
		return 1;
	} else if (result == SGX_DISABLED_REBOOT_REQUIRED) {
		printf("The software enable has been performed on this system and\n");
		printf("Intel SGX will be enabled after the system is rebooted.\n");
		return 1;
	} else if (result != SGX_DISABLED_SCI_AVAILABLE) {
		printf("I couldn't make sense of your system.\n");
		fprintf(stderr, "sgx_cap_get_status returned 0x%04x\n", result);
		return 1;
	} else {
		printf("Unknown result code.\n");
		fprintf(stderr, "sgx_cap_get_status returned 0x%04x\n", result);
		return 1;
	}
}
