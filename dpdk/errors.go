// Copyright 2022 OnMetal authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dpdk

import (
	"errors"
	"fmt"
)

const (
	BAD_REQUEST     = 101
	NOT_FOUND       = 201
	ALREADY_EXISTS  = 202
	WRONG_TYPE      = 203
	BAD_IPVER       = 204
	NO_VM           = 205
	NO_VNI          = 206
	ITERATOR        = 207
	OUT_OF_MEMORY   = 208
	LIMIT_REACHED   = 209
	ROUTE_EXISTS    = 301
	ROUTE_NOT_FOUND = 302
	ROUTE_INSERT    = 303
	ROUTE_BAD_PORT  = 304
	ROUTE_RESET     = 305
	DNAT_NO_DATA    = 321
	DNAT_CREATE     = 322
	DNAT_EXISTS     = 323
	SNAT_NO_DATA    = 341
	SNAT_CREATE     = 342
	SNAT_EXISTS     = 343
	VNI_INIT4       = 361
	VNI_INIT6       = 362
	VNI_FREE4       = 363
	VNI_FREE6       = 364
	PORT_START      = 381
	PORT_STOP       = 382
	VNF_INSERT      = 401
	VM_HANDLE       = 402
	NO_BACKIP       = 421
	NO_LB           = 422
	NO_DROP_SUPPORT = 441

	SERVER_ERROR = 2
)

var ErrServerError = fmt.Errorf("server error")

type StatusError struct {
	errorCode int32
	message   string
}

func (s *StatusError) Message() string {
	return s.message
}

func (s *StatusError) ErrorCode() int32 {
	return s.errorCode
}

func (s *StatusError) Error() string {
	if s.message != "" {
		return fmt.Sprintf("[error code %d] %s", s.errorCode, s.message)
	}
	return fmt.Sprintf("error code %d", s.errorCode)
}

func IsStatusErrorCode(err error, errorCodes ...int32) bool {
	statusError := &StatusError{}
	if !errors.As(err, &statusError) {
		return false
	}

	for _, errorCode := range errorCodes {
		if statusError.ErrorCode() == errorCode {
			return true
		}
	}
	return false
}

func IgnoreStatusErrorCode(err error, errorCode int32) error {
	if IsStatusErrorCode(err, errorCode) {
		return nil
	}
	return err
}
