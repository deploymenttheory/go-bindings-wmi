//go:build windows

package wmi

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/wmi"
)

// ErrEventTimeout reports that no event arrived within the wait passed to
// EventSubscription.Next.
var ErrEventTimeout = errors.New("wmi: event wait timed out")

// EventSubscription is a live WQL event subscription (semisynchronous).
// Poll it with Next; Close cancels it. Like all Service operations it is
// bound to the connecting goroutine.
type EventSubscription struct {
	enum *wmi.IEnumWbemClassObject
}

// SubscribeEvents starts a WQL event query, e.g.
//
//	SELECT * FROM __InstanceCreationEvent WITHIN 2 WHERE TargetInstance ISA 'Win32_Process'
//
// Intrinsic events carry the instance in their TargetInstance property,
// decoded to a nested Row; extrinsic events carry their own properties.
func (s *Service) SubscribeEvents(wql string) (*EventSubscription, error) {
	lang := foundation.SysAllocString("WQL")
	defer foundation.SysFreeString(lang)
	query := foundation.SysAllocString(wql)
	defer foundation.SysFreeString(query)

	var enum *wmi.IEnumWbemClassObject
	if err := s.services.ExecNotificationQuery(lang, query,
		wbemFlagForwardOnly|wbemFlagReturnImmediately, nil, &enum); err != nil {
		return nil, fmt.Errorf("wmi: ExecNotificationQuery(%q): %w", wql, err)
	}
	return &EventSubscription{enum: enum}, nil
}

// Next waits up to the given duration for the next event. It returns
// ErrEventTimeout when the wait elapses (poll again), and io.EOF when the
// subscription has ended.
func (e *EventSubscription) Next(wait time.Duration) (Row, error) {
	obj, status, err := nextObject(e.enum, int32(wait.Milliseconds()))
	if err != nil {
		return nil, err
	}
	switch status {
	case enumTimedOut:
		return nil, ErrEventTimeout
	case enumDone:
		return nil, io.EOF
	}
	defer obj.Release()
	return decodeObject(obj)
}

// Close cancels the subscription.
func (e *EventSubscription) Close() {
	if e.enum != nil {
		e.enum.Release()
		e.enum = nil
	}
}
