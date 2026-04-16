// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package metalbond

import "strings"

func IsAlreadySubscribedToVNIError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already subscribed to vni")
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
	return strings.Contains(strings.ToLower(err.Error()), "already unsubscribed from vni")
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
	return strings.Contains(strings.ToLower(err.Error()), "not subscribed to vni")
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
	return strings.Contains(strings.ToLower(err.Error()), "nexthop already exists")
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
	return strings.Contains(strings.ToLower(err.Error()), "vni does not exist")
}

func IsDestinationNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "destination does not exist")
}

func IsNextHopNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "nexthop does not exist")
}

func IgnoreNextHopNotFoundError(err error) error {
	if IsNextHopNotFoundError(err) || IsVNINotFoundError(err) || IsDestinationNotFoundError(err) {
		return nil
	}
	return err
}
