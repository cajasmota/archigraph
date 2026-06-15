//go:build !darwin && !linux && !windows

package service

import "errors"

var errUnsupported = errors.New("grafel service install is not supported on this platform; use 'grafel daemon start --foreground'")

func install(_ Options) (StatusInfo, error) { return StatusInfo{}, errUnsupported }
func uninstall(_ Options) error             { return errUnsupported }
func status(_ Options) (StatusInfo, error)  { return StatusInfo{}, errUnsupported }
