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
	_ADD_IFACE                  = 100
	_ADD_IFACE_IPV6_FORMAT      = 101
	ADD_IFACE_HANDLE_ERR        = 102
	ADD_IFACE_LPM4_ERR          = 104
	ADD_IFACE_LPM6_ERR          = 105
	ADD_IFACE_ROUTE4_ERR        = 106
	ADD_IFACE_ROUTE6_ERR        = 107
	ADD_IFACE_NO_VFS            = 108
	ADD_IFACE_ALREADY_ALLOCATED = 109
	ADD_IFACE_BAD_DEVICE_NAME   = 110
	ADD_IFACE_VNF_ERR           = 111
	ADD_IFACE_PORT_START_ERR    = 112
	_DEL_IFACE                  = 150
	DEL_IFACE_NOT_FOUND         = 151
	GET_IFACE_NOT_FOUND         = 171
	_LIST_IFACES                = 200
	_ADD_ROUTE                  = 250
	ADD_ROUTE_FAIL4             = 251
	ADD_ROUTE_FAIL6             = 252
	ADD_ROUTE_NO_VM             = 253
	DEL_ROUTE                   = 300
	GET_NAT_ITER_ERR            = 349
	ADD_VIP_NO_SNAT_DATA        = 350
	ADD_VIP_IP_EXISTS           = 351
	ADD_VIP_SNAT_KEY_ERR        = 352
	ADD_VIP_SNAT_ALLOC          = 353
	ADD_VIP_SNAT_DATA_ERR       = 354
	_ADD_NAT                    = 355
	_DEL_NAT                    = 356
	_ADD_NAT_NONLOCAL           = 357
	_ADD_NAT_INVALID_PORT       = 358
	ADD_NAT_NO_SNAT_DATA        = 359
	_DEL_NAT_NONLOCAL           = 360
	_DEL_NAT_INVALID_PORT       = 361
	DEL_NAT_NOT_FOUND           = 362
	ADD_NAT_IP_EXISTS           = 363
	ADD_NAT_SNAT_KEY_ERR        = 364
	ADD_NAT_SNAT_ALLOC          = 365
	ADD_NAT_SNAT_DATA_ERR       = 366
	DEL_NAT_ALREADY_DELETED     = 367
	GET_NATINFO_NO_IPV6_SUPPORT = 369
	ADD_NEIGHNAT_WRONGTYPE      = 370
	DEL_NEIGHNAT_WRONGTYPE      = 371
	ADD_NEIGHNAT_ALREADY_EXISTS = 372
	ADD_NEIGHNAT_ALLOC          = 373
	DEL_NEIGHNAT_NOT_FOUND      = 374
	_GET_NEIGHNAT_UNDER_IPV6    = 375
	GET_NATINFO_WRONGTYPE       = 376
	ADD_NATVIP_VNF_ERR          = 377
	_ADD_DNAT                   = 400
	ADD_DNAT_IP_EXISTS          = 401
	ADD_DNAT_KEY_ERR            = 402
	ADD_DNAT_ALLOC              = 403
	ADD_DNAT_DATA_ERR           = 404
	DEL_VIP_NO_VM               = 450
	DEL_NATVIP_NO_SNAT          = 451
	GET_NATVIP_NO_VM            = 500
	GET_NATVIP_NO_IP_SET        = 501
	ADD_LBVIP_BACKIP_ERR        = 550
	_ADD_LBVIP_NO_VNI           = 551
	ADD_LBVIP_UNSUPP_IP         = 552
	DEL_LBVIP_BACKIP_ERR        = 600
	_DEL_LBVIP_NO_VNI           = 601
	DEL_LBVIP_UNSUPP_IP         = 602
	_ADD_PREFIX                 = 650
	ADD_PREFIX_NO_VM            = 651
	ADD_PREFIX_ROUTE            = 652
	ADD_PREFIX_VNF_ERR          = 653
	DEL_PREFIX_ROUTE_ERR        = 700
	DEL_PREFIX_NO_VM            = 701
	INIT_RESET_ERR              = 710
	ADD_LB_UNSUPP_IP            = 750
	ADD_LB_CREATE_ERR           = 751
	ADD_LB_VNF_ERR              = 752
	ADD_LB_ROUTE_ERR            = 753
	DEL_LB_ID_ERR               = 755
	DEL_LB_BACK_IP_ERR          = 756
	DEL_LB_ROUTE_ERR            = 757
	GET_LB_ID_ERR               = 760
	GET_LB_BACK_IP_ERR          = 761
	ADD_FWRULE_NO_VM            = 800
	ADD_FWRULE_ALLOC_ERR        = 801
	ADD_FWRULE_NO_DROP_SUPPORT  = 802
	ADD_FWRULE_ID_EXISTS        = 803
	GET_FWRULE_NO_VM            = 810
	GET_FWRULE_NOT_FOUND        = 811
	DEL_FWRULE_NO_VM            = 820
	DEL_FWRULE_NOT_FOUND        = 821
)

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
