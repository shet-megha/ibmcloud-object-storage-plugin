package kube

import (
	"bytes"
	"os/exec"
)

func Apply(yaml string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewBufferString(yaml)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	println(string(out))
	return nil
}

func Delete(yaml string) error {
	cmd := exec.Command("kubectl", "delete", "-f", "-")
	cmd.Stdin = bytes.NewBufferString(yaml)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	println(string(out))
	return nil
}
