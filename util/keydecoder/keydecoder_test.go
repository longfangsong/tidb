package keydecoder

import (
	"testing"

	"github.com/pingcap/parser/model"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/table/tables"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/codec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustNewCommonHandle(t *testing.T, values ...interface{}) *kv.CommonHandle {
	encoded, err := codec.EncodeKey(new(stmtctx.StatementContext), nil, types.MakeDatums(values...)...)
	require.Nil(t, err)

	ch, err := kv.NewCommonHandle(encoded)
	require.Nil(t, err)

	return ch
}

func TestDecodeKey(t *testing.T) {
	table.MockTableFromMeta = tables.MockTableFromMeta
	tableInfo1 := &model.TableInfo{
		ID:   1,
		Name: model.NewCIStr("table1"),
		Indices: []*model.IndexInfo{
			{ID: 1, Name: model.NewCIStr("index1"), State: model.StatePublic},
		},
	}
	tableInfo2 := &model.TableInfo{ID: 2, Name: model.NewCIStr("table2")}
	stubTableInfos := []*model.TableInfo{tableInfo1, tableInfo2}
	stubInfoschema := infoschema.MockInfoSchema(stubTableInfos)

	decodedKey, err := DecodeKey([]byte{
		't',
		// table id = 1
		0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		'_',
		'r',
		// int handle, value = 1
		0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	}, stubInfoschema)
	assert.Nil(t, err)
	assert.Equal(t, decodedKey.DbID, int64(0))
	assert.Equal(t, decodedKey.DbName, "test")
	assert.Equal(t, decodedKey.TableID, int64(1))
	assert.Equal(t, decodedKey.TableName, "table1")
	assert.Equal(t, decodedKey.HandleType, IntHandle)
	assert.Equal(t, decodedKey.IsPartitionHandle, false)
	assert.Equal(t, decodedKey.HandleValue, "1")

	ch := mustNewCommonHandle(t, 100, "abc")
	encodedCommonKey := ch.Encoded()
	key := []byte{
		't',
		// table id = 2
		0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
		'_',
		'r',
	}
	key = append(key, encodedCommonKey...)

	decodedKey, err = DecodeKey(key, stubInfoschema)
	assert.Nil(t, err)
	assert.Equal(t, decodedKey.DbID, int64(0))
	assert.Equal(t, decodedKey.DbName, "test")
	assert.Equal(t, decodedKey.TableID, int64(2))
	assert.Equal(t, decodedKey.TableName, "table2")
	assert.Equal(t, decodedKey.HandleType, CommonHandle)
	assert.Equal(t, decodedKey.IsPartitionHandle, false)
	assert.Equal(t, decodedKey.HandleValue, "{100, abc}")

	values := types.MakeDatums("abc", 1)
	sc := &stmtctx.StatementContext{}
	encodedValue, err := codec.EncodeKey(sc, nil, values...)
	key = []byte{
		't',
		// table id = 1
		0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		'_',
		'i',
		// index id = 1
		0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	}
	key = append(key, encodedValue...)

	decodedKey, err = DecodeKey(key, stubInfoschema)
	assert.Nil(t, err)
	assert.Equal(t, decodedKey.DbID, int64(0))
	assert.Equal(t, decodedKey.DbName, "test")
	assert.Equal(t, decodedKey.TableID, int64(1))
	assert.Equal(t, decodedKey.TableName, "table1")
	assert.Equal(t, decodedKey.IndexID, int64(1))
	assert.Equal(t, decodedKey.IndexName, "index1")
	assert.Equal(t, decodedKey.IndexValues, []string{"abc", "1"})
}
