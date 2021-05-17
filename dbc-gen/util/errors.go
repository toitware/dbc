// Copyright (C) 2021 Toitware ApS. All rights reserved.

package util

func FirstError(errors ...error) error {
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}
