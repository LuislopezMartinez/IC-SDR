//go:build !windows

package omnirig

import "errors"

func (client *Client) run() {
	client.finish(errors.New("Omni-Rig solo está disponible en Windows"))
}
