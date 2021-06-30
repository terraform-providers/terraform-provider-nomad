package helper

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type StateWriter struct {
	d    *schema.ResourceData
	mErr *multierror.Error
}

func NewStateWriter(d *schema.ResourceData) *StateWriter {
	return &StateWriter{d: d}
}

func (sw *StateWriter) Set(key string, value interface{}) {
	err := sw.d.Set(key, value)
	if err != nil {
		sw.mErr = multierror.Append(sw.mErr, fmt.Errorf("failed to set %q: %v", key, err))
	}
}

func (sw *StateWriter) Error() error {
	return sw.mErr.ErrorOrNil()
}
