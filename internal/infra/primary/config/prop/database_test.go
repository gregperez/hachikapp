package prop

import (
	"reflect"
	"testing"
)

func TestGetDatabaseProperty(t *testing.T) {
	tests := []struct {
		name string
		want *DatabaseProperty
	}{
		{name: "#1 GetDatabaseProperty", want: &DatabaseProperty{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetDatabaseProperty(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDatabaseProperty() = %v, want %v", got, tt.want)
			}
		})
	}
}
