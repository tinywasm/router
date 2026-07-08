package mock

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
)

// Call records an invocation of a Caller operation.
type Call struct {
	Op   string
	Args model.Encodable
}

// Caller fakes the call-side contract, recording operations and allowing
// canned responses to be configured.
type Caller struct {
	Calls []Call

	// CannedResult is passed to the Call callback.
	CannedResult []byte
	// CannedError is passed to the Call callback.
	CannedError error
}

func (c *Caller) Call(op string, args model.Encodable, callback func(result []byte, err error)) {
	c.Calls = append(c.Calls, Call{Op: op, Args: args})
	if callback != nil {
		callback(c.CannedResult, c.CannedError)
	}
}

func (c *Caller) Dispatch(op string, args model.Encodable) {
	c.Calls = append(c.Calls, Call{Op: op, Args: args})
}

var _ router.Caller = (*Caller)(nil)
