//go:build integration

package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"example.com/app/internal/platform/api/gen"
)

func TestAuditLog(t *testing.T) {
	r := newRig(t, true)
	ctx := context.Background()
	analyst := r.As(t, "analyst")

	// Правка веса: запись «было → стало» с автором.
	if code, resp := patch(t, analyst, false, `{"weights":{"noauto":0.6}}`); code != http.StatusOK {
		t.Fatalf("PATCH: %d %s", code, resp)
	}
	// Предпросмотр ничего не сохраняет и в журнал не попадает.
	if code, resp := patch(t, analyst, true, `{"weights":{"noauto":0.7}}`); code != http.StatusOK {
		t.Fatalf("dryRun: %d %s", code, resp)
	}
	// Правка счёта, загрузка снимка по служебному токену, неудачный вход.
	bal := int64(12345)
	if res, err := analyst.PatchAccountWithResponse(ctx, 1, gen.AccountPatch{Balance: &bal}); err != nil || res.StatusCode() != http.StatusOK {
		t.Fatalf("PATCH account: %v %d %s", err, res.StatusCode(), res.Body)
	}
	r.reload(t)
	bad, err := r.Anonymous(t).LoginWithResponse(ctx, gen.LoginRequest{Login: "analyst", Password: "wrong"})
	if err != nil || bad.StatusCode() != http.StatusUnauthorized {
		t.Fatalf("неверный пароль: %v %d", err, bad.StatusCode())
	}

	// Анонимный вход с огромным логином не раздувает запись: автор усекается.
	long := strings.Repeat("я", 5000)
	if lg, err := r.Anonymous(t).LoginWithResponse(ctx, gen.LoginRequest{Login: long, Password: "x"}); err != nil || lg.StatusCode() != http.StatusUnauthorized {
		t.Fatalf("длинный логин: %v %d", err, lg.StatusCode())
	}

	res, err := analyst.ListAuditWithResponse(ctx, &gen.ListAuditParams{})
	if err != nil || res.JSON200 == nil {
		t.Fatalf("GET audit: %v %d %s", err, res.StatusCode(), res.Body)
	}
	byAction := map[gen.AuditEntryAction]gen.AuditEntry{}
	for _, e := range res.JSON200.Items {
		if _, ok := byAction[e.Action]; !ok {
			byAction[e.Action] = e
		}
	}
	set, ok := byAction[gen.SettingsChanged]
	if !ok || set.Actor != "analyst" || len(set.Changes) != 1 ||
		set.Changes[0].Field != "weights.noauto" || set.Changes[0].From != "0.55" || set.Changes[0].To != "0.6" {
		t.Errorf("запись настроек: %+v", set)
	}
	if n := countAction(res.JSON200.Items, gen.SettingsChanged); n != 1 {
		t.Errorf("записей настроек %d, ждали 1 (dryRun не пишется)", n)
	}
	acc, ok := byAction[gen.AccountChanged]
	if !ok || len(acc.Changes) != 1 || acc.Changes[0].Field != "balance" || acc.Changes[0].To != "12345" {
		t.Errorf("запись счёта: %+v", acc)
	}
	if snap, ok := byAction[gen.SnapshotLoaded]; !ok || snap.Actor != "служебный токен" {
		t.Errorf("запись снимка: %+v", snap)
	}
	if lf, ok := byAction[gen.LoginFailed]; !ok || lf.Actor != "analyst" {
		t.Errorf("неудачный вход: %+v", lf)
	}
	if _, ok := byAction[gen.Login]; !ok {
		t.Error("вход analyst не записан")
	}
	for _, e := range res.JSON200.Items {
		if n := len([]rune(e.Actor)); n > 64 {
			t.Errorf("автор длиной %d рун, ждали не больше 64", n)
		}
		for _, s := range append([]string{e.Summary, e.Actor}, changeText(e.Changes)...) {
			if strings.Contains(s, "wrong") || strings.Contains(s, "@") {
				t.Errorf("в журнале лишнее: %q", s)
			}
		}
	}

	// Пагинация.
	limit := 2
	page, err := analyst.ListAuditWithResponse(ctx, &gen.ListAuditParams{Limit: &limit})
	if err != nil || page.JSON200 == nil || len(page.JSON200.Items) != 2 || page.JSON200.Total != res.JSON200.Total {
		t.Errorf("страница: %v %+v", err, page.JSON200)
	}
}

func countAction(items []gen.AuditEntry, a gen.AuditEntryAction) (n int) {
	for _, e := range items {
		if e.Action == a {
			n++
		}
	}
	return n
}

func changeText(cs []gen.AuditChange) (out []string) {
	for _, c := range cs {
		out = append(out, c.Field, c.From, c.To)
	}
	return out
}

func TestAuditPermissions(t *testing.T) {
	r := newRig(t, false)
	ctx := context.Background()
	for role, want := range map[string]int{"marketer": http.StatusForbidden, "analyst": http.StatusOK, "admin": http.StatusOK, "jury": http.StatusOK} {
		res, err := r.As(t, role).ListAuditWithResponse(ctx, &gen.ListAuditParams{})
		if err != nil || res.StatusCode() != want {
			t.Errorf("%s: %v %d, ждали %d", role, err, res.StatusCode(), want)
		}
	}
	res, err := r.Anonymous(t).ListAuditWithResponse(ctx, &gen.ListAuditParams{})
	if err != nil || res.StatusCode() != http.StatusUnauthorized {
		t.Errorf("без сессии: %v %d", err, res.StatusCode())
	}
}
