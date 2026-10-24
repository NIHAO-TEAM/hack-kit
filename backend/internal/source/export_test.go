package source

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ApplyStreamEvent — эталон для тестов: событие, применённое к снимку целиком, как раньше.
func ApplyStreamEvent(s *Snapshot, ev StreamEvent) error { return applyStreamEvent(s, ev) }

// IngestOutcome — итог одного события при прогоне через приёмник.
type IngestOutcome struct {
	Seq int
	Err error
	// Version — версия данных после пачки; у отказа 0.
	Version int64
}

// IngestRun — итог прогона событий через приёмник.
type IngestRun struct {
	Outcomes   []IngestOutcome
	BatchSizes []int   // сколько событий в каждой записанной пачке
	Published  []int64 // версии, о которых объявлено source.changed
	Duplicates int
}

// RunIngest прогоняет события с заданными номерами через приёмник и ждёт записи всех.
func RunIngest(ctx context.Context, pool *pgxpool.Pool, base Snapshot, seqs []int, evs []StreamEvent) IngestRun {
	var run IngestRun
	g := newEventWriter(pool, newIngestState(&base), time.Now,
		func(v int64) { run.Published = append(run.Published, v) },
		func(res []ingestResult) {
			run.BatchSizes = append(run.BatchSizes, len(res))
			for _, r := range res {
				run.Outcomes = append(run.Outcomes, IngestOutcome{Seq: r.seq, Err: r.err, Version: r.version})
			}
		})
	g.start(ctx)
	for i, ev := range evs {
		g.submit(ctx, seqs[i], ev)
	}
	g.finish()
	run.Duplicates = g.duplicates()
	return run
}

// BatchTxSyncCommit открывает транзакцию пачки и возвращает synchronous_commit внутри неё и на
// обычном соединении того же пула после отката.
func BatchTxSyncCommit(ctx context.Context, pool *pgxpool.Pool) (inside, outside string, err error) {
	tx, err := beginBatchTx(ctx, pool)
	if err != nil {
		return "", "", err
	}
	if err = tx.QueryRow(ctx, "SHOW synchronous_commit").Scan(&inside); err != nil {
		_ = tx.Rollback(ctx)
		return "", "", err
	}
	if err = tx.Rollback(ctx); err != nil {
		return "", "", err
	}
	err = pool.QueryRow(ctx, "SHOW synchronous_commit").Scan(&outside)
	return inside, outside, err
}
