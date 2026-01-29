package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/data/repos"
	"github.com/kova98/feedgrep.api/notifiers"
	"github.com/pkg/errors"
)

type Notifier struct {
	matchRepo *repos.MatchRepo
	usersRepo *repos.UserRepo
	mailer    *notifiers.Mailer
}

func NewNotifier(mailer *notifiers.Mailer, matchRepo *repos.MatchRepo, usersRepo *repos.UserRepo) *Notifier {
	return &Notifier{
		matchRepo: matchRepo,
		usersRepo: usersRepo,
		mailer:    mailer,
	}
}

func (n *Notifier) Start(ctx context.Context) {
	if err := n.notifyUsers(); err != nil {
		slog.Error("notify users:", "error", err)
	}

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := n.notifyUsers(); err != nil {
					slog.Error("notify users:", "error", err)
				}
			}
		}
	}()
}

func (n *Notifier) notifyUsers() error {
	unnotified, err := n.matchRepo.GetUnnotifiedMatches()
	if err != nil {
		return errors.Wrap(err, "notify users: get unnotified matches")
	}
	if len(unnotified) == 0 {
		return nil
	}

	// Group matches by user
	// Collect all matches for each user
	// Decide to send single email or digest based on number of matches
	userMatches := make(map[uuid.UUID][]data.Match)
	userIDs := make([]uuid.UUID, 0, len(unnotified))
	for _, match := range unnotified {
		userMatches[match.UserID] = append(userMatches[match.UserID], match)
		userIDs = append(userIDs, match.UserID)
	}

	u, err := n.usersRepo.GetUsersByIDs(userIDs)
	if err != nil {
		return errors.Wrap(err, "notify users: get users by IDs")
	}
	users := make(map[uuid.UUID]data.User)
	for _, user := range u {
		users[user.ID] = user
	}

	for userID, matches := range userMatches {
		user, ok := users[userID]
		if !ok {
			slog.Error("notify users: user not found", "userID", userID)
			continue
		}

		if len(matches) == 1 {
			mail, err := n.mailer.RedditMatchEmail(user.Email, matches[0])
			if err != nil {
				slog.Error("notify users: create email", "userID", userID, "error", err)
				continue
			}
			if err = n.mailer.Send(mail); err != nil {
				slog.Error("notify users: send match notification", "userID", userID, "error", err)
				continue
			}
			if err = n.matchRepo.MarkNotified([]int64{int64(matches[0].ID)}, time.Now()); err != nil {
				slog.Error("notify users: mark match as notified", "userID", userID, "error", err)
			}
			continue
		}

		digest, err := n.mailer.RedditDigestEmail(user.Email, matches)
		if err != nil {
			slog.Error("notify users: create digest email", "userID", userID, "error", err)
			continue
		}
		err = n.mailer.Send(digest)
		if err != nil {
			slog.Error("notify users: send digest notification", "userID", userID, "error", err)
			continue
		}

		matchIDs := make([]int64, 0, len(matches))
		for _, match := range matches {
			matchIDs = append(matchIDs, int64(match.ID))
		}
		err = n.matchRepo.MarkNotified(matchIDs, time.Now())
		if err != nil {
			slog.Error("failed to mark matches as notified", "userID", userID, "error", err)
		}
	}

	return nil
}
