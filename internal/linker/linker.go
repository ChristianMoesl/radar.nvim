package linker

import (
	"regexp"
	"strings"

	"radar.nvim/internal/protocol"
)

var ticketPattern = regexp.MustCompile(`[A-Z][A-Z0-9]+-[0-9]+`)

type Input struct {
	Items    []protocol.Item
	Entities []protocol.Entity
}

func Link(input Input) []protocol.Item {
	items := cloneItems(input.Items)
	items = attachEntities(items, input.Entities)
	items = append(items, standaloneEntities(items, input.Entities)...)
	return items
}

func cloneItems(items []protocol.Item) []protocol.Item {
	cloned := make([]protocol.Item, len(items))
	copy(cloned, items)
	return cloned
}

func attachEntities(items []protocol.Item, entities []protocol.Entity) []protocol.Item {
	for i := range items {
		itemKeys := keysForItem(items[i])
		for _, entity := range entities {
			if matchesAny(itemKeys, keysForEntity(entity)) {
				items[i].Entities = appendEntity(items[i].Entities, entity)
			}
		}
	}
	return items
}

func standaloneEntities(items []protocol.Item, entities []protocol.Entity) []protocol.Item {
	attached := map[string]bool{}
	for _, item := range items {
		for _, entity := range item.Entities {
			attached[entity.ID] = true
		}
	}

	standalone := make([]protocol.Item, 0)
	for _, entity := range entities {
		if attached[entity.ID] || ignoredEntity(entity) {
			continue
		}
		standalone = append(standalone, itemFromEntity(entity))
	}
	return standalone
}

func itemFromEntity(entity protocol.Entity) protocol.Item {
	reason := entity.Source + " " + entity.Kind
	return protocol.Item{
		ID:        entity.ID,
		Kind:      entity.Source + "_" + entity.Kind,
		Title:     entity.Title,
		Repo:      entity.Repo,
		URL:       entity.URL,
		Attention: "in_progress",
		Reason:    reason,
		Entities:  []protocol.Entity{entity},
	}
}

func appendEntity(entities []protocol.Entity, entity protocol.Entity) []protocol.Entity {
	for _, existing := range entities {
		if existing.ID == entity.ID {
			return entities
		}
	}
	return append(entities, entity)
}

func keysForItem(item protocol.Item) []string {
	values := []string{item.ID, item.Title, item.Repo, item.URL}
	for _, entity := range item.Entities {
		values = append(values, valuesForEntity(entity)...)
	}
	return extractKeys(values...)
}

func keysForEntity(entity protocol.Entity) []string {
	return extractKeys(valuesForEntity(entity)...)
}

func valuesForEntity(entity protocol.Entity) []string {
	values := []string{entity.ID, entity.Title, entity.Branch, entity.Path, entity.Repo, entity.URL}
	if entity.Metadata != nil {
		values = append(values, entity.Metadata["ticket"], entity.Metadata["key"])
	}
	return values
}

func extractKeys(values ...string) []string {
	keys := make([]string, 0)
	seen := map[string]bool{}
	for _, value := range values {
		for _, match := range ticketPattern.FindAllString(strings.ToUpper(value), -1) {
			if !seen[match] {
				seen[match] = true
				keys = append(keys, match)
			}
		}
	}
	return keys
}

func matchesAny(left []string, right []string) bool {
	for _, l := range left {
		for _, r := range right {
			if l == r {
				return true
			}
		}
	}
	return false
}

func ignoredEntity(entity protocol.Entity) bool {
	return entity.Source == "git" && entity.Kind == "worktree" && ignoredBranch(entity.Branch)
}

func ignoredBranch(branch string) bool {
	switch branch {
	case "", "main", "master", "develop", "dev":
		return true
	default:
		return false
	}
}
