package db

import (
	"context"
	"errors"
	"strings"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/types"

	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/sourcegraph/pkg/api"
	"github.com/sourcegraph/sourcegraph/pkg/db/dbconn"
	"github.com/sourcegraph/sourcegraph/pkg/trace"
)

type savedSearches struct{}

func (s *savedSearches) ListAll(ctx context.Context) (_ []api.SavedQuerySpecAndConfig, err error) {
	if Mocks.SavedSearches.ListAll != nil {
		return Mocks.SavedSearches.ListAll(ctx)
	}

	tr, ctx := trace.New(ctx, "db.SavedSearches.ListAll", "")
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	q := sqlf.Sprintf(`SELECT id, description, query, notify_owner, notify_slack, owner_kind, user_id, org_id, slack_webhook_url FROM saved_searches`)
	rows, err := dbconn.Global.QueryContext(ctx, q.Query(sqlf.PostgresBindVar))
	if err != nil {
		return nil, err
	}
	var savedSearches []api.SavedQuerySpecAndConfig
	for rows.Next() {
		var sq api.SavedQuerySpecAndConfig
		if err := rows.Scan(&sq.Config.Key, &sq.Config.Description, &sq.Config.Query, &sq.Config.Notify, &sq.Config.NotifySlack, &sq.Config.OwnerKind, &sq.Config.UserID, &sq.Config.OrgID, &sq.Config.SlackWebhookURL); err != nil {
			return nil, err
		}
		sq.Spec.Key = sq.Config.Key
		if sq.Config.OwnerKind == "user" {
			sq.Spec.Subject.User = sq.Config.UserID
		} else if sq.Config.OwnerKind == "org" {
			sq.Spec.Subject.Org = sq.Config.OrgID
		}
		savedSearches = append(savedSearches, sq)
	}
	return savedSearches, nil
}

func (s *savedSearches) GetSavedSearchByID(ctx context.Context, id string) (*api.SavedQuerySpecAndConfig, error) {
	var savedSearch api.SavedQuerySpecAndConfig

	err := dbconn.Global.QueryRowContext(ctx, `SELECT id, description, query, notify_owner, notify_slack, owner_kind, user_id, org_id, slack_webhook_url FROM saved_searches WHERE id=$1`, id).Scan(&savedSearch.Config.Key, &savedSearch.Config.Description, &savedSearch.Config.Query, &savedSearch.Config.Notify, &savedSearch.Config.NotifySlack, &savedSearch.Config.OwnerKind, &savedSearch.Config.UserID, &savedSearch.Config.OrgID, &savedSearch.Config.SlackWebhookURL)
	savedSearch.Spec.Key = savedSearch.Config.Key
	if savedSearch.Config.UserID != nil {
		savedSearch.Spec.Subject = api.SettingsSubject{User: savedSearch.Config.UserID}
	} else if savedSearch.Config.OrgID != nil {
		savedSearch.Spec.Subject = api.SettingsSubject{Org: savedSearch.Config.OrgID}
	}

	if err != nil {
		return nil, err
	}
	return &savedSearch, err
}

func (s *savedSearches) Create(ctx context.Context, newSavedSearch *types.SavedSearch) (savedQuery *api.ConfigSavedQuery, err error) {
	if Mocks.SavedSearches.Create != nil {
		return Mocks.SavedSearches.Create(ctx, newSavedSearch)
	}

	if newSavedSearch.ID != "" {
		return nil, errors.New("newSavedSearch.ID must be empty string")
	}

	tr, ctx := trace.New(ctx, "db.SavedSearches.Create", "")
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	savedQuery = &api.ConfigSavedQuery{
		Description: newSavedSearch.Description,
		Query:       newSavedSearch.Query,
		Notify:      newSavedSearch.Notify,
		NotifySlack: newSavedSearch.NotifySlack,
		OwnerKind:   newSavedSearch.OwnerKind,
		UserID:      newSavedSearch.UserID,
		OrgID:       newSavedSearch.OrgID,
	}

	if err := dbconn.Global.QueryRowContext(ctx, `INSERT INTO saved_searches(description, query, notify_owner, notify_slack, owner_kind, user_id, org_id) VALUES($1, $2, $3, $4, $5, $6, $7) RETURNING id`, newSavedSearch.Description, newSavedSearch.Query, newSavedSearch.Notify, newSavedSearch.NotifySlack, strings.ToLower(newSavedSearch.OwnerKind), newSavedSearch.UserID, newSavedSearch.OrgID).Scan(&savedQuery.Key); err != nil {
		return nil, err
	}
	return savedQuery, nil
}

func (s *savedSearches) Update(ctx context.Context, savedSearch *types.SavedSearch) (savedQuery *api.ConfigSavedQuery, err error) {
	if Mocks.SavedSearches.Update != nil {
		return Mocks.SavedSearches.Update(ctx, savedSearch)
	}

	tr, ctx := trace.New(ctx, "db.SavedSearches.Update", "")
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	savedQuery = &api.ConfigSavedQuery{
		Description: savedSearch.Description,
		Query:       savedSearch.Query,
		Notify:      savedSearch.Notify,
		NotifySlack: savedSearch.NotifySlack,
		OwnerKind:   savedSearch.OwnerKind,
		UserID:      savedSearch.UserID,
		OrgID:       savedSearch.OrgID,
	}

	fieldUpdates := []*sqlf.Query{
		sqlf.Sprintf("updated_at=now()"),
		sqlf.Sprintf("description=%s", savedSearch.Description),
		sqlf.Sprintf("query=%s", savedSearch.Query),
		sqlf.Sprintf("notify_owner=%t", savedSearch.Notify),
		sqlf.Sprintf("notify_slack=%t", savedSearch.NotifySlack),
		sqlf.Sprintf("owner_kind=%s", strings.ToLower(savedSearch.OwnerKind)),
		sqlf.Sprintf("user_id=%v", savedSearch.UserID),
		sqlf.Sprintf("org_id=%v", savedSearch.OrgID),
	}

	updateQuery := sqlf.Sprintf(`UPDATE saved_searches SET %s WHERE ID=%v RETURNING id`, sqlf.Join(fieldUpdates, ", "), savedSearch.ID)
	if err := dbconn.Global.QueryRowContext(ctx, updateQuery.Query(sqlf.PostgresBindVar), updateQuery.Args()...).Scan(&savedQuery.Key); err != nil {
		return nil, err
	}
	return savedQuery, nil
}

func (s *savedSearches) Delete(ctx context.Context, id string) (err error) {
	if Mocks.SavedSearches.Delete != nil {
		return Mocks.SavedSearches.Delete(ctx, id)
	}

	tr, ctx := trace.New(ctx, "db.SavedSearches.Delete", "")
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()
	_, err = dbconn.Global.ExecContext(ctx, `DELETE FROM saved_searches WHERE ID=$1`, id)
	if err != nil {
		return err
	}
	return nil
}
