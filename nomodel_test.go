package cep_test

// TestHandleReachesNoModel (design s3-3-1 §3.2 MUST 12; S3.2b-1's L2; QA's T12): no value reachable from a Runtime
// or an AppHandle (func values excluded) has a type from internal/model.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vorsar/cep"
)

func fcWalk(t *testing.T, v reflect.Value, path string, seen map[uintptr]bool) {
	if !v.IsValid() {
		return
	}
	ty := v.Type()
	if strings.HasSuffix(ty.PkgPath(), "/internal/model") {
		t.Errorf("%s holds a %s (internal/model is reachable from the facade)", path, ty)
		return
	}
	switch v.Kind() {
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return
		}
		if v.Kind() == reflect.Ptr {
			if seen[v.Pointer()] {
				return
			}
			seen[v.Pointer()] = true
		}
		fcWalk(t, v.Elem(), path+"→"+ty.String(), seen)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			fcWalk(t, v.Field(i), path+"."+ty.Field(i).Name, seen)
		}
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.Len() > 0 {
			if seen[v.Pointer()] {
				return
			}
			seen[v.Pointer()] = true
		}
		for i := 0; i < v.Len(); i++ {
			fcWalk(t, v.Index(i), path+"[]", seen)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			fcWalk(t, k, path+"{key}", seen)
			fcWalk(t, v.MapIndex(k), path+"{}", seen)
		}
	}
}

func TestHandleReachesNoModel(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	_, bind := fcSink("RiskAlerts")
	h := fcDeploy(t, ctx, r, "09", fcExample(t, "09-params-and-quoting.cep"), bind, cep.WithParam("threshold", "0.5"))
	seen := map[uintptr]bool{}
	fcWalk(t, reflect.ValueOf(r), "Runtime", seen)
	fcWalk(t, reflect.ValueOf(h), "AppHandle", seen)
}
