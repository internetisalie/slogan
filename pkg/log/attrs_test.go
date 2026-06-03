package log

import (
	"log/slog"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_setAttrsAtPath(t *testing.T) {
	type args struct {
		parentAttrs []slog.Attr
		groups      []string
		attrs       []slog.Attr
	}
	tests := []struct {
		name string
		args args
		want []slog.Attr
	}{
		{
			name: "Flat",
			args: args{
				parentAttrs: []slog.Attr{slog.Int("a", 1)},
				groups:      []string{},
				attrs:       []slog.Attr{slog.Int("b", 2)},
			},
			want: []slog.Attr{slog.Int("a", 1), slog.Int("b", 2)},
		},
		{
			name: "MissingParent",
			args: args{
				parentAttrs: []slog.Attr{slog.Int("a", 1)},
				groups:      []string{"b"},
				attrs:       []slog.Attr{slog.Int("c", 2)},
			},
			want: []slog.Attr{slog.Int("a", 1), slog.Group("b", slog.Int("c", 2))},
		},
		{
			name: "MissingParents",
			args: args{
				parentAttrs: []slog.Attr{slog.Int("a", 1)},
				groups:      []string{"b", "c"},
				attrs:       []slog.Attr{slog.Int("d", 2)},
			},
			want: []slog.Attr{slog.Int("a", 1), slog.Group("b", slog.Group("c", slog.Int("d", 2)))},
		},
		{
			name: "ExistingParents",
			args: args{
				parentAttrs: []slog.Attr{slog.Group("a", slog.Group("b", slog.Int("c", 1)))},
				groups:      []string{"a", "b"},
				attrs:       []slog.Attr{slog.Int("d", 2)},
			},
			want: []slog.Attr{slog.Group("a", slog.Group("b", slog.Int("c", 1), slog.Int("d", 2)))},
		},
		{
			name: "ExistingKey",
			args: args{
				parentAttrs: []slog.Attr{slog.Int("a", 1)},
				groups:      []string{},
				attrs:       []slog.Attr{slog.Int("a", 2)},
			},
			want: []slog.Attr{slog.Int("a", 2)},
		},
		{
			name: "Merge",
			args: args{
				parentAttrs: []slog.Attr{slog.Group("a", slog.Int("b", 1))},
				groups:      []string{},
				attrs:       []slog.Attr{slog.Group("a", slog.Int("c", 2))},
			},
			want: []slog.Attr{slog.Group("a", slog.Int("b", 1), slog.Int("c", 2))},
		},
		{
			name: "MergeOverwrite",
			args: args{
				parentAttrs: []slog.Attr{slog.Group("a", slog.Int("b", 1))},
				groups:      []string{},
				attrs:       []slog.Attr{slog.Group("a", slog.Int("b", 2), slog.Int("c", 1))},
			},
			want: []slog.Attr{
				slog.Group("a", slog.Int("b", 2), slog.Int("c", 1)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SetAttrsAtPath(tt.args.parentAttrs, tt.args.groups, tt.args.attrs)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddGroup(t *testing.T) {
	groups := []string{"a", "b"}
	assert.Equal(t, []string{"a", "b", "c"}, AddGroup(groups, "c"))
}

func TestGetValueAtPath(t *testing.T) {
	attrs := []slog.Attr{
		slog.Group("a", slog.Int("b", 1)),
	}
	val, ok := GetValueAtPath(attrs, "a", "b")
	assert.True(t, ok)
	assert.Equal(t, int64(1), val.Int64())

	_, ok = GetValueAtPath(attrs, "a", "c")
	assert.False(t, ok)

	_, ok = GetValueAtPath(attrs, "x", "y")
	assert.False(t, ok)
}

func TestMapAttrs(t *testing.T) {
	m := map[string]any{"a": 1, "b": "c"}
	attrs := MapAttrs(m)
	assert.Len(t, attrs, 2)
}

func TestSliceAttrs(t *testing.T) {
	s := []any{1, "a"}
	attrs := SliceAttrs(reflect.ValueOf(s))
	assert.Len(t, attrs, 2)
}

func TestAttr(t *testing.T) {
	a := Attr("foo", 123)
	assert.Equal(t, "foo", a.Key)
	assert.Equal(t, int64(123), a.Value.Int64())
}

func TestValue(t *testing.T) {
	v := Value(456)
	assert.Equal(t, int64(456), v.Int64())
}

func TestReflectValue(t *testing.T) {
	v := ReflectValue(reflect.ValueOf("hello"))
	assert.Equal(t, "hello", v.String())
	
	v = ReflectValue(reflect.Value{})
	assert.Equal(t, "<nil>", v.String())
}
