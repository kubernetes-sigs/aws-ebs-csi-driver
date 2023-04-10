package template

import (
	"fmt"
	"strings"
	"text/template"

	"k8s.io/klog/v2"
)

type PVProps struct {
	PVCName      string
	PVCNamespace string
	PVName       string
}

type VolumeSnapshotProps struct {
	VolumeSnapshotName        string
	VolumeSnapshotNamespace   string
	VolumeSnapshotContentName string
}

func Evaluate(tm []string, props interface{}, warnOnly bool) (map[string]string, error) {
	md := make(map[string]string)
	for _, s := range tm {
		st := strings.SplitN(s, "=", 2)
		if len(st) != 2 {
			return nil, fmt.Errorf("the key-value pair doesn't contain a value (string: %s)", s)
		}

		key, value := st[0], st[1]

		t := template.New("tmpl").Funcs(template.FuncMap(newFuncMap()))
		val, err := execTemplate(value, props, t)
		if err != nil {
			if warnOnly {
				klog.InfoS("Unable to interpolate value", "key", key, "value", value, "err", err)
			} else {
				return nil, err
			}
		} else {
			md[key] = val
		}
	}
	return md, nil
}

func execTemplate(value string, props interface{}, t *template.Template) (string, error) {
	tmpl, err := t.Parse(value)
	if err != nil {
		return "", err
	}

	b := new(strings.Builder)
	err = tmpl.Execute(b, props)
	if err != nil {
		return "", err
	}

	return b.String(), nil
}
