//
// go.notify :: notify.go
//
//   Copyright (c) 2017-2019 Akinori Hattori <hattya@gmail.com>
//
//   SPDX-License-Identifier: MIT
//

// Package notify provides an interface for notifications.
package notify

import "errors"

var ErrEvent = errors.New("go.notify: unknown event")

// Icon represents an icon. Its value is dependent on each implementation.
type Icon interface{}

// Notifier is an interface for notifications.
type Notifier interface {
	// Close closes the Notifier.
	Close() error

	// Register registers the named event to the Notifier. The keys and values
	// of the opts are dependent on each implementation.
	//
	// Notifier may use the icon for notifications.
	Register(event string, icon Icon, opts map[string]interface{}) error

	// Notify notifies the named event by the specified title and body.
	Notify(event, title, body string) error

	// Sys returns the implementation of the Notifier.
	Sys() interface{}
}
