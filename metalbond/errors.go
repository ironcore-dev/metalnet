// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

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
