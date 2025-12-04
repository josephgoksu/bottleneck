package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// GraphQL Response Structures
type GraphQLResponse struct {
	Data struct {
		Repository struct {
			PullRequests struct {
				Nodes    []GRPCPullRequest `json:"nodes"`
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"pullRequests"`
		} `json:"repository"`
	} `json:"data"`
}

type GRPCPullRequest struct {
	Number    int       `json:"number"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	MergedAt  time.Time `json:"mergedAt"`
	Title     string    `json:"title"`
	Additions int       `json:"additions"`
	Deletions int       `json:"deletions"`
	Author    struct {
		Login string `json:"login"`
	}
	Reviews struct {
		Nodes []struct {
			CreatedAt time.Time `json:"createdAt"`
			Author    struct {
				Login string `json:"login"`
			} `json:"author"`
		}
	}
	ReviewRequests struct {
		Nodes []struct {
			RequestedReviewer struct {
				Login string `json:"login"`
			} `json:"requestedReviewer"`
		}
	} `json:"reviewRequests"`
	Files struct {
		Nodes []struct {
			Path string `json:"path"`
		} `json:"nodes"`
	} `json:"files"`
}

type PullRequest struct {
	Number        int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	MergedAt      time.Time
	FirstReviewAt *time.Time
	Author        string
	Title         string
	Size          int
	FilePaths     []string
	Reviewers     []string // Who actually reviewed
	Requested     []string // Who is requested (for open PRs)
}

func main() {
	// 1. Parse Flags
	excludeOutliers := flag.Bool("exclude-outliers", false, "Exclude top and bottom 5% of outliers")
	limit := flag.Int("limit", 100, "Max number of PRs to fetch (max 100 for GraphQL)")
	reqTimeout := flag.Duration("timeout", 30*time.Second, "Timeout for each API request")
	reqDelay := flag.Duration("delay", 200*time.Millisecond, "Delay between API requests to avoid rate limits")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: go run main.go [flags] <owner/repo>")
		flag.PrintDefaults()
		os.Exit(1)
	}
	repo := args[0]
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		fmt.Println("Error: Repo must be in format owner/repo")
		os.Exit(1)
	}
	owner, name := parts[0], parts[1]

	// 2. Fetch Data (Merged PRs for Stats)
	fmt.Printf("🔍 Fetching merged PRs for %s (limit %d)...\n", repo, *limit)
	mergedPRs, err := fetchPRs(owner, name, *limit, "MERGED", *reqTimeout, *reqDelay)
	if err != nil {
		fmt.Printf("Error fetching Merged PRs: %v\n", err)
		os.Exit(1)
	}

	// 3. Fetch Data (Open PRs for Ghosts/Stale) - Limit 100 is usually enough for active backlog
	fmt.Printf("🔍 Fetching open PRs for analysis (limit 100)...")
	openPRs, err := fetchPRs(owner, name, 100, "OPEN", *reqTimeout, *reqDelay)
	if err != nil {
		fmt.Printf("Error fetching Open PRs: %v\n", err)
		// We continue even if open PRs fail, just to show merged stats
	}

	if len(mergedPRs) == 0 && len(openPRs) == 0 {
		fmt.Println("No PRs found.")
		return
	}

	// --- Merged PR Analysis ---
	if len(mergedPRs) > 0 {
		// Filter Outliers (Optional)
		if *excludeOutliers {
			originalCount := len(mergedPRs)
			mergedPRs = filterOutliers(mergedPRs)
			fmt.Printf("✂️  Outlier filtering active. Reduced from %d to %d PRs.\n", originalCount, len(mergedPRs))
		}
		fmt.Println(strings.Repeat("-", 60))

		printGeneralStats(mergedPRs)
		fmt.Println(strings.Repeat("-", 60))
		printReviewStats(mergedPRs)
		fmt.Println(strings.Repeat("-", 60))
		printSizeAnalysis(mergedPRs)
		fmt.Println(strings.Repeat("-", 60))
		printHotspots(mergedPRs)
		fmt.Println(strings.Repeat("-", 60))
		printLongTailAuthors(mergedPRs)
		fmt.Println(strings.Repeat("-", 60))
		printTrends(mergedPRs)
		fmt.Println(strings.Repeat("-", 60))
		printForecast(mergedPRs)
		fmt.Println(strings.Repeat("-", 60))
		printHistogram(mergedPRs)
		fmt.Println(strings.Repeat("-", 60))

		// NEW: Hero Syndrome (Uses Merged Data)
		printHeroAnalysis(mergedPRs)
		fmt.Println(strings.Repeat("-", 60))
	}

	// --- Open PR Analysis ---
	if len(openPRs) > 0 {
		// NEW: Stale PRs
		printStaleAnalysis(openPRs)
		fmt.Println(strings.Repeat("-", 60))

		// NEW: Ghost Reviewers
		printGhostAnalysis(openPRs)
		fmt.Println(strings.Repeat("-", 60))
	}
}

// Generic Fetch Function for both OPEN and MERGED
func fetchPRs(owner, name string, limit int, state string, timeout time.Duration, delay time.Duration) ([]PullRequest, error) {
	var allPRs []PullRequest
	var cursor string

	// GraphQL Query Template
	// We fetch reviews (for heroes) and reviewRequests (for ghosts)
	queryTmpl := `
query {
  repository(owner: "%s", name: "%s") {
    pullRequests(%s) {
      nodes {
        number
        createdAt
        updatedAt
        mergedAt
        title
        additions
        deletions
        author { login }
        reviews(first: 10) {
          nodes {
            createdAt
            author { login }
          }
        }
        reviewRequests(first: 10) {
          nodes {
            requestedReviewer {
              ... on User { login }
            }
          }
        }
        files(first: 5) {
          nodes { path }
        }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
}`

	for len(allPRs) < limit {
		if len(allPRs) > 0 {
			time.Sleep(delay)
		}

		remaining := limit - len(allPRs)
		toFetch := 100
		if remaining < 100 {
			toFetch = remaining
		}

		// Order by Created DESC for Merged, Updated DESC for Open (usually better for stale checks)
		orderBy := "CREATED_AT"
		if state == "OPEN" {
			orderBy = "UPDATED_AT"
		}

		args := fmt.Sprintf("first: %d, states: %s, orderBy: {field: %s, direction: DESC}", toFetch, state, orderBy)
		if cursor != "" {
			args += fmt.Sprintf(`, after: "%s"`, cursor)
		}

		query := fmt.Sprintf(queryTmpl, owner, name, args)

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "gh", "api", "graphql", "-f", fmt.Sprintf("query=%s", query))
		output, err := cmd.Output()

		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("request timed out after %v", timeout)
		}
		if err != nil {
			return nil, err
		}

		var resp GraphQLResponse
		if err := json.Unmarshal(output, &resp); err != nil {
			return nil, err
		}

		nodes := resp.Data.Repository.PullRequests.Nodes
		if len(nodes) == 0 {
			break
		}

		for _, node := range nodes {
			pr := PullRequest{
				Number:    node.Number,
				CreatedAt: node.CreatedAt,
				UpdatedAt: node.UpdatedAt,
				MergedAt:  node.MergedAt,
				Author:    node.Author.Login,
				Title:     node.Title,
				Size:      node.Additions + node.Deletions,
			}

			// Process Reviews
			if len(node.Reviews.Nodes) > 0 {
				// First review time
				t := node.Reviews.Nodes[0].CreatedAt
				pr.FirstReviewAt = &t

				// Collect Reviewers
				seen := make(map[string]bool)
				for _, r := range node.Reviews.Nodes {
					if r.Author.Login != "" && r.Author.Login != pr.Author && !seen[r.Author.Login] {
						pr.Reviewers = append(pr.Reviewers, r.Author.Login)
						seen[r.Author.Login] = true
					}
				}
			}

			// Process Requested Reviewers
			for _, req := range node.ReviewRequests.Nodes {
				if req.RequestedReviewer.Login != "" {
					pr.Requested = append(pr.Requested, req.RequestedReviewer.Login)
				}
			}

			// Process Files
			for _, f := range node.Files.Nodes {
				pr.FilePaths = append(pr.FilePaths, f.Path)
			}

			allPRs = append(allPRs, pr)
		}

		if !resp.Data.Repository.PullRequests.PageInfo.HasNextPage {
			break
		}
		cursor = resp.Data.Repository.PullRequests.PageInfo.EndCursor
	}

	return allPRs, nil
}

// --- Stats Functions ---

func printHeroAnalysis(prs []PullRequest) {
	fmt.Println("🦸 HERO SYNDROME DETECTOR")
	fmt.Println("   • Concept: Identifies developers reviewing a disproportionate amount of code.")
	fmt.Println("   • Why:     Heroes are single points of failure. If they leave or burn out, velocity crashes.")
	fmt.Println("")

	reviewCounts := make(map[string]int)
	totalReviews := 0

	for _, pr := range prs {
		for _, reviewer := range pr.Reviewers {
			reviewCounts[reviewer]++
			totalReviews++
		}
	}

	if totalReviews == 0 {
		fmt.Println("   No reviews found in this dataset.")
		return
	}

	// Sort
	type Reviewer struct {
		Name  string
		Count int
	}
	var heroes []Reviewer
	for name, count := range reviewCounts {
		heroes = append(heroes, Reviewer{name, count})
	}
	sort.Slice(heroes, func(i, j int) bool { return heroes[i].Count > heroes[j].Count })

	// Check for heroes (>30% of total reviews is usually a warning sign, >50% is critical)
	foundRisk := false
	for _, h := range heroes {
		percentage := float64(h.Count) / float64(totalReviews) * 100

		if percentage > 20.0 { // Lower threshold to show top contributors generally
			riskLevel := ""
			if percentage > 50 {
				riskLevel = "🚨 CRITICAL RISK"
				foundRisk = true
			} else if percentage > 30 {
				riskLevel = "⚠️  High Load"
				foundRisk = true
			} else {
				riskLevel = "✅ Healthy"
			}

			fmt.Printf("   %s: %d reviews (%.1f%%) - %s\n", h.Name, h.Count, percentage, riskLevel)
		}
	}

	if !foundRisk {
		fmt.Println("   ✅ Load is well-distributed. No single reviewer is a bottleneck.")
	}
}

func printStaleAnalysis(prs []PullRequest) {
	fmt.Println("📉 STALE PR DETECTOR (The Graveyard)")
	fmt.Println("   • Concept: Open PRs that haven't been touched in >7 days.")
	fmt.Println("   • Why:     Stale PRs rot, cause conflicts, and discourage the team.")
	fmt.Println("")

	now := time.Now()
	staleThreshold := 7 * 24 * time.Hour
	staleCount := 0

	for _, pr := range prs {
		if now.Sub(pr.UpdatedAt) > staleThreshold {
			staleCount++
			days := int(now.Sub(pr.UpdatedAt).Hours() / 24)
			fmt.Printf("   💀 #%d (%s) by %s - %d days inactive\n", pr.Number, limitString(pr.Title, 40), pr.Author, days)
		}
	}

	if staleCount == 0 {
		fmt.Println("   ✅ Clean board! No stale PRs found.")
	} else {
		fmt.Printf("\n   Action: Ping these authors or close the PRs.\n")
	}
}

func printGhostAnalysis(prs []PullRequest) {
	fmt.Println("👻 GHOST REVIEWER DETECTOR")
	fmt.Println("   • Concept: Reviewers requested >48h ago who haven't responded.")
	fmt.Println("   • Why:     Silent blocking. The PR owner is waiting for a notification that never comes.")
	fmt.Println("")

	now := time.Now()
	ghostThreshold := 48 * time.Hour

ghosts := make(map[string]int)

	for _, pr := range prs {
		// Only check PRs that are older than 48h, otherwise the request is fresh
		if now.Sub(pr.CreatedAt) > ghostThreshold {
			for _, reviewer := range pr.Requested {
				// Simple logic: If you are still in "Requested", you haven't reviewed yet.
				// (GitHub moves you from Requested -> Reviews once you submit)
				ghosts[reviewer]++
			}
		}
	}

	if len(ghosts) == 0 {
		fmt.Println("   ✅ No ghosts found. Everyone is responding (or PRs are new).")
		return
	}

	// Sort
	var names []string
	for n := range ghosts {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return ghosts[names[i]] > ghosts[names[j]] })

	for _, name := range names {
		count := ghosts[name]
		fmt.Printf("   👻 %s: Blocking %d PRs (>48h)\n", name, count)
	}
}

func limitString(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// --- Existing Analysis Functions (Preserved) ---

func filterOutliers(prs []PullRequest) []PullRequest {
	if len(prs) < 4 {
		return prs
	}
	sort.Slice(prs, func(i, j int) bool { return prs[i].MergedAt.Sub(prs[i].CreatedAt) < prs[j].MergedAt.Sub(prs[j].CreatedAt) })
	cut := int(float64(len(prs)) * 0.05)
	if cut == 0 {
		cut = 1
	}
	return prs[cut : len(prs)-cut]
}

func printGeneralStats(prs []PullRequest) {
	var totalDuration time.Duration
	var durations []time.Duration

	for _, pr := range prs {
		d := pr.MergedAt.Sub(pr.CreatedAt)
		durations = append(durations, d)
		totalDuration += d
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

	avg := totalDuration / time.Duration(len(prs))
	var median time.Duration
	mid := len(durations) / 2
	if len(durations)%2 == 0 {
		median = (durations[mid-1] + durations[mid]) / 2
	} else {
		median = durations[mid]
	}

	fmt.Println("📊 GENERAL STATISTICS")
	fmt.Println("   • Concept: Measures the total lifecycle of a Pull Request from creation to merge.")
	fmt.Println("   • Why:     High average vs median indicates outliers dragging the team down. This is your baseline velocity.")
	fmt.Println("")

	fmt.Printf("   Count:   %d\n", len(prs))
	fmt.Printf("   Average: %s\n", humanizeDuration(avg))
	fmt.Printf("   Median:  %s\n", humanizeDuration(median))
	fmt.Printf("   Min:     %s\n", humanizeDuration(durations[0]))
	fmt.Printf("   Max:     %s\n", humanizeDuration(durations[len(durations)-1]))
}

func printReviewStats(prs []PullRequest) {
	var totalWait, totalReview time.Duration
	var countWait, countReview int

	for _, pr := range prs {
		if pr.FirstReviewAt != nil {
			wait := pr.FirstReviewAt.Sub(pr.CreatedAt)
			review := pr.MergedAt.Sub(*pr.FirstReviewAt)
			if wait < 0 {
				wait = 0
			}
			if review < 0 {
				review = 0
			}
			totalWait += wait
			totalReview += review
			countWait++
			countReview++
		}
	}

	fmt.Println("🚦 REVIEW EFFICIENCY")
	fmt.Println("   • Concept: Splits time into 'Waiting for Review' vs 'Active Review Process'.")
	fmt.Println("   • Why:     Helps distinguish between a Triage problem (ignoring PRs) and a Complexity problem (hard to approve).")
	fmt.Println("")

	if countWait == 0 {
		fmt.Println("   No reviews detected (Direct merges?).")
	} else {
		avgWait := totalWait / time.Duration(countWait)
		avgReview := totalReview / time.Duration(countReview)
		fmt.Printf("   Avg Time to First Review:   %s (Triage Speed)\n", humanizeDuration(avgWait))
		fmt.Printf("   Avg Review to Merge:        %s (Coding/Fixing Speed)\n", humanizeDuration(avgReview))
	}
}

func printSizeAnalysis(prs []PullRequest) {
	fmt.Println("📐 SIZE vs SPEED ANALYSIS")
	fmt.Println("   • Concept: Correlation between lines of code changed and merge duration.")
	fmt.Println("   • Why:     Determines if 'Big PRs' are the bottleneck or if the process is slow regardless of size.")
	fmt.Println("")

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	n := float64(len(prs))

	for _, pr := range prs {
		size := float64(pr.Size)                                   // X (Lines changed)
		duration := float64(pr.MergedAt.Sub(pr.CreatedAt).Hours()) // Y (Hours)

		sumX += size
		sumY += duration
		sumXY += size * duration
		sumX2 += size * size
		sumY2 += duration * duration
	}

	numerator := n*sumXY - sumX*sumY
	denominator := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))

	correlation := 0.0
	if denominator != 0 {
		correlation = numerator / denominator
	}

	fmt.Printf("   Correlation Coeff: %.2f  (Range: -1.0 to +1.0)\n", correlation)

	if correlation > 0.5 {
		fmt.Println("   🚨 RESULT: Strong Positive Correlation (> 0.5)")
		fmt.Println("      Insight: Larger PRs take significantly longer to merge.")
		fmt.Println("      Action:  Break tasks into smaller, atomic PRs to speed up velocity.")
	} else if correlation > 0.3 {
		fmt.Println("   ⚠️  RESULT: Moderate Correlation (0.3 - 0.5)")
		fmt.Println("      Insight: Size is a factor, but not the only one.")
		fmt.Println("      Action:  Encourage smaller PRs, but also look for process bottlenecks.")
	} else {
		fmt.Println("   ✅ RESULT: Weak/No Correlation (< 0.3)")
		fmt.Println("      Insight: Small PRs are getting stuck just as often as huge ones.")
		fmt.Println("      Action:  Your bottleneck is likely PROCESS (Triage/CI/Availability), not code size.")
	}
}

func printHotspots(prs []PullRequest) {
	fmt.Println("🔥 DIRECTORY HOTSPOTS (Avg Merge Time)")
	fmt.Println("   • Concept: Average merge time grouped by root directory.")
	fmt.Println("   • Why:     Identifies parts of the codebase that are 'swamps'—hard to review, prone to debate, or lacking owners.")
	fmt.Println("")

	type DirStat struct {
		TotalDuration time.Duration
		Count         int
	}
	stats := make(map[string]*DirStat)

	for _, pr := range prs {
		seenDirs := make(map[string]bool)
		duration := pr.MergedAt.Sub(pr.CreatedAt)

		for _, path := range pr.FilePaths {
			parts := strings.Split(path, "/")
			root := parts[0]
			if len(parts) == 1 {
				root = "(root files)"
			}

			if !seenDirs[root] {
				if _, exists := stats[root]; !exists {
					stats[root] = &DirStat{}
				}
				stats[root].TotalDuration += duration
				stats[root].Count++
				seenDirs[root] = true
			}
		}
	}

	var dirs []string
	for d := range stats {
		dirs = append(dirs, d)
	}

	sort.Slice(dirs, func(i, j int) bool { return (stats[dirs[i]].TotalDuration / time.Duration(stats[dirs[i]].Count)) > (stats[dirs[j]].TotalDuration / time.Duration(stats[dirs[j]].Count)) })

	for i, d := range dirs {
		if i >= 5 {
			break
		}
		s := stats[d]
		avg := s.TotalDuration / time.Duration(s.Count)
		fmt.Printf("   %-20s: %s (avg over %d PRs)\n", d, humanizeDuration(avg), s.Count)
	}
}

func printLongTailAuthors(prs []PullRequest) {
	fmt.Println("🐌 LONG TAIL CONTRIBUTORS (Handling the Slowest 10%)")
	fmt.Println("   • Concept: Authors frequently found in the slowest 10% of merges.")
	fmt.Println("   • Why:     These devs might be tackling the hardest problems, or they need help breaking down tasks. Prevents burnout.")
	fmt.Println("")

	sortedPRs := make([]PullRequest, len(prs))
	copy(sortedPRs, prs)
	sort.Slice(sortedPRs, func(i, j int) bool { return sortedPRs[i].MergedAt.Sub(sortedPRs[i].CreatedAt) > sortedPRs[j].MergedAt.Sub(sortedPRs[j].CreatedAt) })

	limit := len(prs) / 10
	if limit == 0 {
		limit = 1
	}
	slowest := sortedPRs[:limit]

	authorCounts := make(map[string]int)
	for _, pr := range slowest {
		authorCounts[pr.Author]++
	}

	var authors []string
	for a := range authorCounts {
		authors = append(authors, a)
	}
	sort.Slice(authors, func(i, j int) bool { return authorCounts[authors[i]] > authorCounts[authors[j]] })

	for i, a := range authors {
		if i >= 5 {
			break
		}
		fmt.Printf("   %-15s: %d slow PRs\n", a, authorCounts[a])
	}
	fmt.Println("   (Note: These authors might be tackling the hardest complexity, not working slowly.)")
}

func printTrends(prs []PullRequest) {
	fmt.Println("📈 MONTHLY TRENDS")
	fmt.Println("   • Concept: Monthly average merge times over the requested period.")
	fmt.Println("   • Why:     Spot if the team is getting faster (🚀) or bogging down (🐢) over time.")
	fmt.Println("")

	type MonthStats struct {
		TotalDuration time.Duration
		Count         int
	}
	stats := make(map[string]*MonthStats)
	var months []string

	for _, pr := range prs {
		m := pr.MergedAt.Format("2006-01")
		if _, exists := stats[m]; !exists {
			stats[m] = &MonthStats{}
			months = append(months, m)
		}
		stats[m].TotalDuration += pr.MergedAt.Sub(pr.CreatedAt)
		stats[m].Count++
	}

	sort.Strings(months)

	var prevAvg time.Duration
	for _, m := range months {
		s := stats[m]
		avg := s.TotalDuration / time.Duration(s.Count)

		trend := ""
		if prevAvg != 0 {
			if avg < prevAvg {
				trend = "🚀"
			} else if avg > prevAvg {
				trend = "🐢"
			} else {
				trend = "➖"
			}
		}
		prevAvg = avg
		fmt.Printf("   %s: %-15s (%2d PRs) %s\n", m, humanizeDuration(avg), s.Count, trend)
	}
}

func printForecast(prs []PullRequest) {
	fmt.Println("🔮 FORECAST (Next 30 Days)")
	fmt.Println("   • Concept: A 3-month moving average projection of merge times.")
	fmt.Println("   • Why:     Predicts where your velocity is heading if current habits continue.")
	fmt.Println("")

	type MonthStat struct {
		Total time.Duration
		Count int
	}
	stats := make(map[string]*MonthStat)
	var months []string

	for _, pr := range prs {
		m := pr.MergedAt.Format("2006-01")
		if _, exists := stats[m]; !exists {
			stats[m] = &MonthStat{}
			months = append(months, m)
		}
		stats[m].Total += pr.MergedAt.Sub(pr.CreatedAt)
		stats[m].Count++
	}
	sort.Strings(months)

	if len(months) < 3 {
		fmt.Println("   (Not enough data for a reliable forecast. Need 3+ months.)")
		return
	}

	last3 := months[len(months)-3:]
	var totalAvg time.Duration

	fmt.Println("   Based on last 3 months:")
	for _, m := range last3 {
		s := stats[m]
		avg := s.Total / time.Duration(s.Count)
		totalAvg += avg
		fmt.Printf("   - %s: %s\n", m, humanizeDuration(avg))
	}

	forecast := totalAvg / 3
	first := stats[last3[0]].Total / time.Duration(stats[last3[0]].Count)
	last := stats[last3[2]].Total / time.Duration(stats[last3[2]].Count)

	trendEmoji := "➡️"
	trendText := "Stable"

	diff := last - first
	threshold := first / 10

	if diff > threshold {
		trendEmoji = "📉"
		trendText = "Slowing Down"
	} else if diff < -threshold {
		trendEmoji = "📈"
		trendText = "Speeding Up"
	}

	fmt.Printf("\n   🎯 PREDICTION: ~%s / PR\n", humanizeDuration(forecast))
	fmt.Printf("   🏁 TREND:      %s %s\n", trendEmoji, trendText)
}

func printHistogram(prs []PullRequest) {
	fmt.Println("📊 MERGE TIME DISTRIBUTION")
	fmt.Println("   • Concept: Distribution of merge times into buckets.")
	fmt.Println("   • Why:     Averages lie. This reveals the 'long tail' of stuck PRs that frustrate the team.")
	fmt.Println("")

	buckets := []struct {
		Label string
		Max   time.Duration
		Count int
	}{
		{"< 1h", time.Hour, 0},
		{"1h - 1d", 24 * time.Hour, 0},
		{"1d - 1w", 7 * 24 * time.Hour, 0},
		{"1w - 1mo", 30 * 24 * time.Hour, 0},
		{"> 1mo", time.Duration(math.MaxInt64), 0},
	}

	maxCount := 0
	for _, pr := range prs {
		d := pr.MergedAt.Sub(pr.CreatedAt)
		for i := range buckets {
			if d < buckets[i].Max {
				buckets[i].Count++
				if buckets[i].Count > maxCount {
					maxCount = buckets[i].Count
				}
				break
			}
		}
	}

	for _, b := range buckets {
		barLen := 0
		if maxCount > 0 {
			barLen = (b.Count * 20) / maxCount
		}
		bar := strings.Repeat("■", barLen)
		fmt.Printf("   %-10s : %-20s (%d)\n", b.Label, bar, b.Count)
	}
}

func humanizeDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}

	days := int(d.Hours()) / 24
	if days < 30 {
		return fmt.Sprintf("%dd %dh", days, int(d.Hours())%24)
	}

	months := days / 30
	remainingDays := days % 30
	if months < 12 {
		return fmt.Sprintf("%dmo %dd", months, remainingDays)
	}

	years := days / 365
	remainingMonths := (days % 365) / 30
	return fmt.Sprintf("%dy %dmo", years, remainingMonths)
}
