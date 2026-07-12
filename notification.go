package config

// Change describes one committed setting change.
type Change struct {
	// Path is the canonical path of the changed setting.
	Path string
	// Old is the previous formatted value.
	Old string
	// New is the committed formatted value.
	New string
	// Revision identifies the configuration revision containing the change.
	Revision uint64
}

// ChangeNotifier receives stable change records after a commit.
type ChangeNotifier interface {
	// NotifyChange receives a committed setting change.
	NotifyChange(Change)
}

// ChangeFunc adapts a function to ChangeNotifier.
type ChangeFunc func(Change)

// NotifyChange implements ChangeNotifier.
func (f ChangeFunc) NotifyChange(change Change) { f(change) }

// Notifier receives a setting after its value changes.
type Notifier interface {
	// Notify receives the changed setting.
	Notify(s *Setting)
}

// NotifyHandle is used to stop notifications of Setting changes
type NotifyHandle struct {
	stopFunc func(any)
}

// Close stops notifications. Calling Close more than once is safe.
func (h *NotifyHandle) Close() error {
	if h.stopFunc == nil {
		return nil
	}

	h.stopFunc(h)

	return nil
}

// NotifyFunc adapts a function to Notifier.
type NotifyFunc func(s *Setting)

// Notify implements Notifier.Notify
func (f NotifyFunc) Notify(s *Setting) {
	f(s)
}
