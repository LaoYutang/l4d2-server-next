package logic

import (
	"errors"
	"sync/atomic"
)

var ErrPluginOperationBusy = errors.New("插件操作正在进行，请稍后重试")

// Runtime operations must not overlap deployment/removal, even when two packages
// contain the same SMX. A fail-fast reservation avoids holding the state lock
// across RCON or recursively acquiring the deployment operation semaphore.
var pluginMutationBusy atomic.Bool

func beginPluginMutation() (func(), error) {
	if !pluginMutationBusy.CompareAndSwap(false, true) {
		return nil, ErrPluginOperationBusy
	}
	return func() { pluginMutationBusy.Store(false) }, nil
}
