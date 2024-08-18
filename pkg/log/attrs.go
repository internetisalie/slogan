package log

import (
	"encoding"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"reflect"
	"strconv"
	"time"

	"github.com/samber/lo"
)

func AddGroup(groups []string, group string) []string {
	result := make([]string, len(groups)+1)
	copy(result, groups)
	result[len(result)-1] = group
	return result
}

func GetValueAtPath(attrs []slog.Attr, p ...string) (slog.Value, bool) {
	here := slog.GroupValue(attrs...)

	for len(p) > 0 {
		if here.Kind() != slog.KindGroup {
			return slog.Value{}, false
		}

		found := false
		for _, attr := range here.Group() {
			if attr.Key == p[0] {
				here = attr.Value
				found = true
				break
			}
		}

		if !found {
			return slog.Value{}, false
		}

		p = p[1:]
	}

	return here, true
}

func SetAttrsAtPath(parentAttrs []slog.Attr, groups []string, attrs []slog.Attr) []slog.Attr {
	if len(groups) == 0 {
		return MergeAttrs(parentAttrs, attrs)
	}

	newAttrs := make([]slog.Attr, len(parentAttrs), len(parentAttrs)+len(attrs))
	copy(newAttrs, parentAttrs)

	childIndex := attrIndex(newAttrs, groups[0])
	if childIndex < len(newAttrs) {
		childAttr := newAttrs[childIndex]
		childAttrs := childAttr.Value.Group()
		newChildAttrs := SetAttrsAtPath(childAttrs, groups[1:], attrs)
		newAttrs[childIndex] = slog.Attr{
			Key:   childAttr.Key,
			Value: slog.GroupValue(newChildAttrs...),
		}
	} else {
		for i := len(groups) - 1; i >= 0; i-- {
			attrs = []slog.Attr{slog.Group(groups[i], lo.ToAnySlice(attrs)...)}
		}
		newAttrs = append(newAttrs, attrs...)
	}

	return newAttrs
}

func MergeAttrs(current []slog.Attr, add []slog.Attr) []slog.Attr {
	newAttrs := make([]slog.Attr, len(current), len(current)+len(add))
	copy(newAttrs, current)

	for _, addAttr := range add {
		found := false
		for i, parentAttr := range newAttrs {
			if parentAttr.Key == addAttr.Key {
				// merge or replace existing attribute
				if parentAttr.Value.Kind() == slog.KindGroup && addAttr.Value.Kind() == slog.KindGroup {
					newAttrs[i] = slog.Group(addAttr.Key,
						lo.ToAnySlice(MergeAttrs(parentAttr.Value.Group(), addAttr.Value.Group()))...)
				} else {
					newAttrs[i] = addAttr
				}
				found = true
				break
			}
		}
		if !found {
			// no matching attribute found
			newAttrs = append(newAttrs, addAttr)
		}
	}

	return newAttrs
}

func attrIndex(attrs []slog.Attr, name string) int {
	for i, attr := range attrs {
		if attr.Value.Kind() == slog.KindGroup && attr.Key == name {
			return i
		}
	}
	return len(attrs)
}

func MapAttrs(values map[string]any) []slog.Attr {
	if len(values) == 0 {
		return nil
	}

	results := make([]slog.Attr, 0, len(values))
	for key, value := range values {
		results = append(results, Attr(key, value))
	}
	return results
}

func SliceAttrs(value reflect.Value) []slog.Attr {
	results := make([]slog.Attr, value.Len())
	for i := 0; i < len(results); i++ {
		results[i] = Attr(strconv.Itoa(i), value.Index(i).Interface())
	}
	return results
}

func Attr(k string, v any) slog.Attr {
	if vt, ok := v.(slog.LogValuer); ok {
		return slog.Attr{
			Key:   k,
			Value: Value(vt.LogValue().Any()),
		}
	}
	return slog.Attr{
		Key:   k,
		Value: Value(v),
	}
}

func Value(v any) slog.Value {
	if vt, ok := v.(encoding.TextMarshaler); ok {
		b, _ := vt.MarshalText()
		return slog.StringValue(string(b))
	}
	if vt, ok := v.(fmt.Stringer); ok {
		return slog.StringValue(vt.String())
	}

	switch vt := v.(type) {
	case time.Time:
		return slog.StringValue(vt.UTC().Format(time.RFC3339Nano))
	case time.Duration:
		return slog.StringValue(vt.String())
	case netip.AddrPort:
		return slog.StringValue(vt.String())
	case net.IP:
		return slog.StringValue(vt.String())
	case net.UDPAddr:
		return slog.StringValue(vt.String())
	case int:
		return slog.IntValue(vt)
	case int8:
		return slog.Int64Value(int64(vt))
	case int16:
		return slog.Int64Value(int64(vt))
	case int32:
		return slog.Int64Value(int64(vt))
	case int64:
		return slog.Int64Value(vt)
	case uint:
		return slog.Uint64Value(uint64(vt))
	case uint8:
		return slog.Uint64Value(uint64(vt))
	case uint16:
		return slog.Uint64Value(uint64(vt))
	case uint32:
		return slog.Uint64Value(uint64(vt))
	case uint64:
		return slog.Uint64Value(vt)
	case float32:
		return slog.Float64Value(float64(vt))
	case float64:
		return slog.Float64Value(vt)
	case bool:
		return slog.BoolValue(vt)
	case string:
		return slog.StringValue(vt)
	case map[string]any:
		return slog.GroupValue(MapAttrs(vt)...)
	}

	vrv := reflect.ValueOf(v)
	return ReflectValue(vrv)
}

func ReflectValue(rv reflect.Value) slog.Value {
	var empty reflect.Value
	if rv == empty {
		return slog.StringValue("<nil>")
	}

	rt := rv.Type()

	switch rt.Kind() {
	case reflect.Slice:
		return slog.GroupValue(SliceAttrs(rv)...)
	}

	return slog.AnyValue(rv.Interface())
}
