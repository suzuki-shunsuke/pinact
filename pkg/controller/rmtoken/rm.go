package rmtoken

import (
	"fmt"
)

func (c *Controller) Remove() error {
	if err := c.tokenManager.RemoveToken(); err != nil {
		return fmt.Errorf("remove a GitHub access Token from the secret store: %w", err)
	}
	return nil
}
