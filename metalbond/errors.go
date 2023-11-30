// Copyright 2022 IronCore authors
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

package metalbond

import "strings"

func IsAlreadySubscribedToVNIError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Already subscribed to VNI")
}

func IgnoreAlreadySubscribedToVNIError(err error) error {
	if IsAlreadySubscribedToVNIError(err) {
		return nil
	}
	return err
}

func IsAlreadyUnsubscribedToVNIError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Already unsubscribed from VNI")
}

func IgnoreAlreadyUnsubscribedToVNIError(err error) error {
	if IsAlreadyUnsubscribedToVNIError(err) {
		return nil
	}
	return err
}

// TODO: IsNotSubscribedToVNIError is not yet implemented on metalbond side.
// Verify as soon as it is.

func IsNotSubscribedToVNIError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Not subscribed to VNI")
}

func IgnoreNotSubscribedToVNIError(err error) error {
	if IsNotSubscribedToVNIError(err) {
		return nil
	}
	return err
}

func IsNextHopAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Nexthop already exists")
}

func IgnoreNextHopAlreadyExistsError(err error) error {
	if IsNextHopAlreadyExistsError(err) {
		return nil
	}
	return err
}

func IsVNINotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "VNI does not exist")
}

func IsDestinationNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Destination does not exist")
}

func IsNextHopNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Nexthop does not exist")
}

func IgnoreNextHopNotFoundError(err error) error {
	if IsNextHopNotFoundError(err) || IsVNINotFoundError(err) || IsDestinationNotFoundError(err) {
		return nil
	}
	return err
}
