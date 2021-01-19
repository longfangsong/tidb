package kv

import (
	"fmt"
	"github.com/pingcap/kvproto/pkg/kvrpcpb"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/logutil"
	"sync"
	"time"
)

const (
	TxnStateRunning     = "Running"
	TxnStateLockWaiting = "LockWaiting"
	TxnStateRollingBack = "RollingBack"
	TxnStateCommitting  = "Commiting"
)

type TxnStatus struct {
	TxnId          uint64
	State          string
	StartTime      time.Time
	IsolationLevel IsoLevel
}

func (t *TxnStatus) ToDatum() []types.Datum {
	var isoLevelStr string
	switch t.IsolationLevel {
	case SI:
		isoLevelStr = "Snapshot Isolation"
	case RC:
		isoLevelStr = "Read Committed"
	}
	return types.MakeDatums(t.TxnId, t.State, t.StartTime.String(), isoLevelStr)
}

type TxnStatusCollector struct {
	mu           sync.Mutex
	txns         map[uint64]*TxnStatus
	lastDeadLock *kvrpcpb.Deadlock
}

func (c *TxnStatusCollector) ReportDeadLock(deadlock *kvrpcpb.Deadlock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	logutil.BgLogger().Info(fmt.Sprint(deadlock))
	c.lastDeadLock = &kvrpcpb.Deadlock{
		LockTs:          deadlock.GetLockTs(),
		LockKey:         deadlock.GetLockKey(),
		DeadlockKeyHash: deadlock.GetDeadlockKeyHash(),
	}
}

func (c *TxnStatusCollector) ReportTxnStart(id uint64, isoLevel IsoLevel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.txns[id] = &TxnStatus{
		TxnId:          id,
		State:          TxnStateRunning,
		StartTime:      time.Now(),
		IsolationLevel: isoLevel,
	}
}

func (c *TxnStatusCollector) ReportTxnCommitting(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.txns[id]; !ok {
		c.txns[id] = &TxnStatus{
			TxnId:          id,
			State:          TxnStateCommitting,
			StartTime:      time.Now(),
			IsolationLevel: RC,
		}
	} else {
		c.txns[id].State = TxnStateCommitting
	}
}

func (c *TxnStatusCollector) ReportTxnDone(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.txns, id)
}

var Collector TxnStatusCollector

func init() {
	Collector = TxnStatusCollector{
		mu:   sync.Mutex{},
		txns: make(map[uint64]*TxnStatus),
	}
}

func (c *TxnStatusCollector) ToDatums() [][]types.Datum {
	c.mu.Lock()
	defer c.mu.Unlock()
	var result [][]types.Datum
	for _, status := range c.txns {
		result = append(result, status.ToDatum())
	}
	return result
}

func (c *TxnStatusCollector) DeadLockDatum() []types.Datum {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastDeadLock == nil {
		return nil
	}
	return types.MakeDatums(c.lastDeadLock.LockKey, c.lastDeadLock.DeadlockKeyHash, c.lastDeadLock.LockTs)
}
