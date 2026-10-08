// css.gomuks.app - A user CSS repository for gomuks web.
// Copyright (C) 2026 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/meowlnir/policylist"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

var policies *policySyncer

var policyEventTypes = []event.Type{
	event.StatePolicyUser, event.StateLegacyPolicyUser, event.StateUnstablePolicyUser,
	event.StatePolicyServer, event.StateLegacyPolicyServer, event.StateUnstablePolicyServer,
}

type policySyncer struct {
	mu    sync.RWMutex
	rooms []id.RoomID
	store *policylist.Store
}

func startPolicySync(ctx context.Context) error {
	log := defLog.With().Str("component", "policy sync").Logger()
	ctx = log.WithContext(ctx)
	var rooms []id.RoomID
	for room := range strings.SplitSeq(os.Getenv("POLICY_LIST_ROOMS"), ",") {
		room = strings.TrimSpace(room)
		if room == "" {
			continue
		} else if !strings.HasPrefix(room, "!") {
			return fmt.Errorf("invalid policy list room ID %q", room)
		}
		if !slices.Contains(rooms, id.RoomID(room)) {
			rooms = append(rooms, id.RoomID(room))
		}
	}
	policies = &policySyncer{
		rooms: rooms,
		store: policylist.NewStore(),
	}
	if len(rooms) == 0 {
		log.Debug().Msg("No policy list rooms configured")
		return nil
	}
	log.Info().Any("rooms", rooms).Msg("Preparing policy syncer")
	excludeAll := &mautrix.FilterPart{NotTypes: []event.Type{{Type: "*"}}}
	filterJSON := &mautrix.Filter{
		Presence:    excludeAll,
		AccountData: excludeAll,
		Room: &mautrix.RoomFilter{
			Rooms:       rooms,
			AccountData: excludeAll,
			Ephemeral:   excludeAll,
			State:       &mautrix.FilterPart{Types: policyEventTypes},
			Timeline:    &mautrix.FilterPart{Types: policyEventTypes},
		},
	}
	filter, err := matrixClient.CreateFilter(ctx, filterJSON)
	if err != nil {
		return fmt.Errorf("failed to create policy sync filter: %w", err)
	}
	req := mautrix.ReqSync{
		FilterID:      filter.FilterID,
		SetPresence:   event.PresenceOffline,
		UseStateAfter: true,
	}
	go func() {
		for ctx.Err() == nil {
			resp, err := matrixClient.FullSyncRequest(ctx, req)
			if err == nil {
				err = policies.ProcessResponse(ctx, resp, req.Since)
			}
			if err == nil {
				req.Since = resp.NextBatch
				req.Timeout = 30000
				continue
			} else if ctx.Err() != nil {
				return
			}
			zerolog.Ctx(ctx).Err(err).Msg("Policy list sync failed")
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
		}
	}()
	return nil
}

func (ps *policySyncer) ProcessResponse(ctx context.Context, resp *mautrix.RespSync, since string) error {
	if len(resp.Rooms.Join) == 0 {
		return nil
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	changed := false
	for _, roomID := range ps.rooms {
		room, ok := resp.Rooms.Join[roomID]
		if !ok {
			continue
		}
		var eventMap map[event.Type]map[string]*event.Event
		if since == "" {
			eventMap = make(map[event.Type]map[string]*event.Event)
		}
		var events []*event.Event
		if room.StateAfter != nil {
			events = room.StateAfter.Events
		} else {
			events = append(room.State.Events, room.Timeline.Events...)
		}
		for _, evt := range events {
			if evt.StateKey == nil {
				continue
			}
			evt.RoomID = roomID
			evt.Type.Class = event.StateEventType
			if err := evt.Content.ParseRaw(evt.Type); err != nil && !errors.Is(err, event.ErrContentAlreadyParsed) {
				zerolog.Ctx(ctx).Warn().Err(err).Stringer("event_id", evt.ID).Msg("Failed to parse policy event")
				continue
			}
			if since == "" {
				typeMap, ok := eventMap[evt.Type]
				if !ok {
					typeMap = make(map[string]*event.Event)
					eventMap[evt.Type] = typeMap
				}
				typeMap[evt.GetStateKey()] = evt
			} else {
				added, _ := ps.store.Update(evt)
				changed = changed || (added != nil && !added.Ignored)
			}
		}
		if eventMap != nil {
			ps.store.Add(roomID, eventMap)
			changed = true
		}
	}
	if changed {
		users, err := db.GetContentUsers(ctx)
		if err != nil {
			return fmt.Errorf("failed to get users for policy enforcement: %w", err)
		}
		for _, userID := range users {
			if ps.isBanned(userID) {
				zerolog.Ctx(ctx).Info().Stringer("user_id", userID).Msg("Removing content from banned user")
				if err = db.RemoveUserContent(ctx, userID); err != nil {
					return err
				}
			}
		}
	}
	if since == "" {
		zerolog.Ctx(ctx).Info().Msg("Initial policy list sync complete")
	}
	return nil
}

func (ps *policySyncer) isBanned(userID id.UserID) bool {
	userPolicy := ps.store.MatchUser(ps.rooms, userID).Recommendations().BanOrUnban
	if userPolicy != nil {
		return userPolicy.Recommendation == event.PolicyRecommendationBan
	}
	serverPolicy := ps.store.MatchServer(ps.rooms, userID.Homeserver()).Recommendations().BanOrUnban
	if serverPolicy != nil {
		return serverPolicy.Recommendation == event.PolicyRecommendationBan
	}
	return false
}

func lockPolicyActions(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := readCookie(r)
		if userID == "" || r.URL.Path == "/login" ||
			strings.HasSuffix(r.URL.Path, "/report") ||
			(r.Method != http.MethodPost && r.Method != http.MethodPut) {
			next.ServeHTTP(w, r)
			return
		}
		policies.mu.RLock()
		defer policies.mu.RUnlock()
		next.ServeHTTP(w, r)
	})
}
