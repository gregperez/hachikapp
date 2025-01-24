package database

import (
	"reflect"
	"testing"
)

func TestGetDatabasePostgresPgx4(t *testing.T) {
	tests := []struct {
		name string
		want *PostgresPgx
	}{
		{
			name: "#1 Database process", want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetDatabasePostgresPgx(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDatabasePostgresPgx4() = %v, want %v", got, tt.want)
			}
		})
	}
}

//func TestNewConnectionPostgresPgx4(t *testing.T) {
//	type args struct {
//		prop *properties.DatabaseProperty
//	}
//	tests := []struct {
//		name string
//		args args
//		want *PostgresPgx
//	}{
//		{name: "#1 NewConectionPostgresBun",
//			args: args{prop: &properties.DatabaseProperty{DataSource: properties.DataSource{"localhost", "5432", "test", "test", "test", 10}}}, want: nil},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if got := NewConnectionPostgresPgx4(tt.args.prop); !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("NewConnectionPostgresPgx4() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}

//func TestPostgresPgx4_QueryRow(t *testing.T) {
//	type fields struct {
//		pool *pgxpool.Pool
//		ctx  context.Context
//	}
//	type args struct {
//		sql  string
//		args []interface{}
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//		want   pgx.Row
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			db := &PostgresPgx{
//				pool: tt.fields.pool,
//				ctx:  tt.fields.ctx,
//			}
//			if got := db.QueryRow(tt.args.sql, tt.args.args...); !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("QueryRow() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}

//func TestPostgresPgx4_QueryRows(t *testing.T) {
//	type fields struct {
//		pool     *pgxpool.Pool
//		TimeZone string
//		ctx      context.Context
//	}
//	type args struct {
//		sql  string
//		args []interface{}
//	}
//	tests := []struct {
//		name    string
//		fields  fields
//		args    args
//		want    pgx.Rows
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			db := &PostgresPgx{
//				pool:     tt.fields.pool,
//				ctx:      tt.fields.ctx,
//			}
//			got, err := db.QueryRows(tt.args.sql, tt.args.args...)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("QueryRows() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("QueryRows() got = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}

//func TestPostgresPgx4_Start(t *testing.T) {
//	type fields struct {
//		pool *pgxpool.Pool
//		ctx  context.Context
//	}
//	tests := []struct {
//		name   string
//		fields fields
//	}{
//		{name: "#1 Start", fields: fields{pool: nil, ctx: context.Background()}},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			db := &PostgresPgx{
//				pool: tt.fields.pool,
//				ctx:  tt.fields.ctx,
//			}
//			db.Start()
//		})
//	}
//}
