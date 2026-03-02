package main

import (
	"fmt"
	"os"

	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

func main() {
	root := os.Args[1]
	s, _ := store.LoadStore(root)

	groups := []model.Group{
		{ID: "grp-1", Name: "Frontend", Created: model.Now()},
		{ID: "grp-2", Name: "Backend", Created: model.Now()},
		{ID: "grp-3", Name: "DevOps", Created: model.Now()},
	}
	s.Groups = append(s.Groups, groups...)

	titles := []string{
		"Fix login button alignment", "Add dark mode toggle", "Update navbar responsive layout",
		"Implement search autocomplete", "Fix modal z-index issue", "Add loading skeleton screens",
		"Refactor form validation", "Fix image lazy loading", "Add breadcrumb navigation", "Update footer links",
		"Create user API endpoint", "Fix database connection pool", "Add rate limiting middleware",
		"Implement caching layer", "Fix query N+1 problem", "Add webhook support",
		"Refactor auth middleware", "Fix session timeout handling", "Add bulk import endpoint", "Update error response format",
		"Set up CI pipeline", "Configure staging environment", "Add health check endpoint", "Set up log aggregation",
		"Fix Docker build caching", "Add Terraform modules", "Configure auto-scaling", "Set up monitoring alerts",
		"Fix SSL certificate renewal", "Add backup automation",
	}

	groupIDs := []string{
		"grp-1", "grp-1", "grp-1", "grp-1", "grp-1", "grp-1", "grp-1", "grp-1", "grp-1", "grp-1",
		"grp-2", "grp-2", "grp-2", "grp-2", "grp-2", "grp-2", "grp-2", "grp-2", "grp-2", "grp-2",
		"grp-3", "grp-3", "grp-3", "grp-3", "grp-3", "grp-3", "grp-3", "grp-3", "grp-3", "grp-3",
	}
	statuses := []model.TaskStatus{model.StatusPending, model.StatusInProgress, model.StatusDone}

	for i, title := range titles {
		s.NextID++
		s.Tasks = append(s.Tasks, model.Task{
			ID: fmt.Sprintf("T%d", s.NextID), Title: title, Status: statuses[i%3],
			Tags: []string{}, Group: groupIDs[i], Created: model.Now(), Updated: model.Now(),
		})
	}

	store.SaveStore(root, s)
	fmt.Fprintf(os.Stderr, "Saved %d tasks, %d groups\n", len(s.Tasks), len(s.Groups))
}
