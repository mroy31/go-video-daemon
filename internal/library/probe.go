package library

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"time"
)

type KwArgs map[string]interface{}

func MergeKwArgs(args []KwArgs) KwArgs {
	a := KwArgs{}
	for _, b := range args {
		for c := range b {
			a[c] = b[c]
		}
	}
	return a
}

func (a KwArgs) Copy() KwArgs {
	r := KwArgs{}
	for k := range a {
		r[k] = a[k]
	}
	return r
}

func (a KwArgs) SortedKeys() []string {
	var r []string
	for k := range a {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

func (a KwArgs) GetString(k string) string {
	if v, ok := a[k]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func (a KwArgs) PopString(k string) string {
	if c, ok := a[k]; ok {
		defer delete(a, k)
		return fmt.Sprintf("%v", c)
	}
	return ""
}

func (a KwArgs) HasKey(k string) bool {
	_, ok := a[k]
	return ok
}

func (a KwArgs) GetDefault(k string, defaultV interface{}) interface{} {
	if v, ok := a[k]; ok {
		return v
	}
	return defaultV
}

func (a KwArgs) PopDefault(k string, defaultV interface{}) interface{} {
	if v, ok := a[k]; ok {
		defer delete(a, k)
		return v
	}
	return defaultV
}

func ConvertKwargsToCmdLineArgs(kwargs KwArgs) []string {
	var keys, args []string
	for k := range kwargs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := kwargs[k]
		switch a := v.(type) {
		case string:
			args = append(args, fmt.Sprintf("-%s", k))
			if a != "" {
				args = append(args, a)
			}
		case []string:
			for _, r := range a {
				args = append(args, fmt.Sprintf("-%s", k))
				if r != "" {
					args = append(args, r)
				}
			}
		case []int:
			for _, r := range a {
				args = append(args, fmt.Sprintf("-%s", k))
				args = append(args, strconv.Itoa(r))
			}
		case int:
			args = append(args, fmt.Sprintf("-%s", k))
			args = append(args, strconv.Itoa(a))
		default:
			args = append(args, fmt.Sprintf("-%s", k))
			args = append(args, fmt.Sprintf("%v", a))
		}
	}
	return args
}

// Probe Run ffprobe on the specified file and return a JSON representation of the output.
func Probe(fileName string, kwargs ...KwArgs) (string, error) {
	return ProbeWithTimeout(fileName, 0, MergeKwArgs(kwargs))
}

func ProbeWithTimeout(fileName string, timeOut time.Duration, kwargs KwArgs) (string, error) {
	args := KwArgs{
		"show_format":  "",
		"show_streams": "",
		"of":           "json",
	}

	return ProbeWithTimeoutExec(fileName, timeOut, MergeKwArgs([]KwArgs{args, kwargs}))
}

func ProbeWithTimeoutExec(fileName string, timeOut time.Duration, kwargs KwArgs) (string, error) {
	args := ConvertKwargsToCmdLineArgs(kwargs)
	args = append(args, fileName)
	ctx := context.Background()
	if timeOut > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(context.Background(), timeOut)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	buf := bytes.NewBuffer(nil)
	stdErrBuf := bytes.NewBuffer(nil)
	cmd.Stdout = buf
	cmd.Stderr = stdErrBuf
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("[%s] %w", stdErrBuf.String(), err)
	}
	return buf.String(), nil
}
