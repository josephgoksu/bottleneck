package main

import (
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
		}
	}
	Files struct {
		Nodes []struct {
			Path string `json:"path"`
		} `json:"nodes"`
	} `json:"files"`
}

type PullRequest struct {
	Number        int
	CreatedAt     time.Time
	MergedAt      time.Time
	FirstReviewAt *time.Time // Nil if no review
	Author        string
	Title         string
	Size          int // Additions + Deletions
	FilePaths     []string
}

func main() {
	// 1. Parse Flags
	excludeOutliers := flag.Bool("exclude-outliers", false, "Exclude top and bottom 5% of outliers")
	limit := flag.Int("limit", 100, "Max number of PRs to fetch (max 100 for GraphQL)")
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

	// 2. Fetch Data
	fmt.Printf("🔍 Fetching merged PRs for %s (limit %d)...\n", repo, *limit)
	prs, err := fetchPRsGraphQL(owner, name, *limit)
	if err != nil {
		fmt.Printf("Error fetching PRs: %v\n", err)
		os.Exit(1)
	}

	if len(prs) == 0 {
		fmt.Println("No merged PRs found.")
		return
	}

	// 3. Filter Outliers (Optional)
	if *excludeOutliers {
		originalCount := len(prs)
		prs = filterOutliers(prs)
		fmt.Printf("✂️  Outlier filtering active. Reduced from %d to %d PRs.\n", originalCount, len(prs))
	}
	fmt.Println(strings.Repeat("-", 60))

	// 4. General Stats
	printGeneralStats(prs)
	fmt.Println(strings.Repeat("-", 60))

	// 5. Review Efficiency
	printReviewStats(prs)
	fmt.Println(strings.Repeat("-", 60))

	// 6. Size Correlation (NEW)
	printSizeAnalysis(prs)
	fmt.Println(strings.Repeat("-", 60))

	// 7. Directory Hotspots (NEW)
	printHotspots(prs)
	fmt.Println(strings.Repeat("-", 60))

	// 8. Long Tail Authors (NEW)
	printLongTailAuthors(prs)
	fmt.Println(strings.Repeat("-", 60))

	// 9. Trend Analysis
	printTrends(prs)
	fmt.Println(strings.Repeat("-", 60))

	// 10. Forecast (NEW)
	printForecast(prs)
	fmt.Println(strings.Repeat("-", 60))

	// 11. Histogram
	printHistogram(prs)
}

func printForecast(prs []PullRequest) {
	fmt.Println("🔮 FORECAST (Next 30 Days)")

	// 1. Aggregate by Month
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

	// Need at least 3 months of data for a trend
	if len(months) < 3 {
		fmt.Println("   (Not enough data for a reliable forecast. Need 3+ months.)")
		return
	}

	// 2. Take last 3 months
	last3 := months[len(months)-3:]
	var totalAvg time.Duration

	fmt.Println("   Based on last 3 months:")
	for _, m := range last3 {
		s := stats[m]
		avg := s.Total / time.Duration(s.Count)
		totalAvg += avg
		fmt.Printf("   - %s: %s\n", m, humanizeDuration(avg))
	}

	// 3. Moving Average
	forecast := totalAvg / 3

	// 4. Simple slope check (Are we getting worse?)
	first := stats[last3[0]].Total / time.Duration(stats[last3[0]].Count)
	last := stats[last3[2]].Total / time.Duration(stats[last3[2]].Count)

	trendEmoji := "➡️"
	trendText := "Stable"

	// Threshold: 10% change
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

func fetchPRsGraphQL(owner, name string, limit int) ([]PullRequest, error) {
	var allPRs []PullRequest
	var cursor string

	for len(allPRs) < limit {
		remaining := limit - len(allPRs)
		toFetch := 100
		if remaining < 100 {
			toFetch = remaining
		}

		args := fmt.Sprintf("first: %d, states: MERGED, orderBy: {field: CREATED_AT, direction: DESC}", toFetch)
		if cursor != "" {
			args += fmt.Sprintf(`, after: "%s"`, cursor)
		}

		query := fmt.Sprintf(`
query {
  repository(owner: "%s", name: "%s") {
    pullRequests(%s) {
      nodes {
        number
        createdAt
        mergedAt
        title
        additions
        deletions
        author {
          login
        }
        reviews(first: 1) {
          nodes {
            createdAt
          }
        }
        files(first: 10) {
          nodes {
            path
          }
        }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
}`, owner, name, args)

		cmd := exec.Command("gh", "api", "graphql", "-f", fmt.Sprintf("query=%s", query))
		output, err := cmd.Output()
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				return nil, fmt.Errorf("%s", exitError.Stderr)
			}
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
				MergedAt:  node.MergedAt,
				Author:    node.Author.Login,
				Title:     node.Title,
				Size:      node.Additions + node.Deletions,
			}
			if len(node.Reviews.Nodes) > 0 {
				t := node.Reviews.Nodes[0].CreatedAt
				pr.FirstReviewAt = &t
			}
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

func filterOutliers(prs []PullRequest) []PullRequest {
	if len(prs) < 4 {
		return prs
	}
	sort.Slice(prs, func(i, j int) bool {
		return prs[i].MergedAt.Sub(prs[i].CreatedAt) < prs[j].MergedAt.Sub(prs[j].CreatedAt)
	})
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
	fmt.Println("   (Time from PR Creation -> Merge)")
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
	if countWait == 0 {
		fmt.Println("   No reviews detected (Direct merges?).")
	} else {
		avgWait := totalWait / time.Duration(countWait)
		avgReview := totalReview / time.Duration(countReview)
		fmt.Printf("   Avg Time to First Review:   %s (Triage Speed)\n", humanizeDuration(avgWait))
		fmt.Printf("   Avg Review to Merge:        %s (Coding/Fixing Speed)\n", humanizeDuration(avgReview))
	}
}

// NEW: Regression Analysis (Size vs Time)
func printSizeAnalysis(prs []PullRequest) {
	fmt.Println("📐 SIZE vs SPEED ANALYSIS")
	fmt.Println("   (Does the size of a PR affect how fast it merges?)")

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

	// Pearson Correlation Coefficient
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

// NEW: Hotspot Map
func printHotspots(prs []PullRequest) {
	fmt.Println("🔥 DIRECTORY HOTSPOTS (Avg Merge Time)")

	type DirStat struct {
		TotalDuration time.Duration
		Count         int
	}
	stats := make(map[string]*DirStat)

	for _, pr := range prs {
		seenDirs := make(map[string]bool)
		duration := pr.MergedAt.Sub(pr.CreatedAt)

		for _, path := range pr.FilePaths {
			// Extract root directory (e.g., "pkg/api/foo.go" -> "pkg")
			parts := strings.Split(path, "/")
			root := parts[0]
			if len(parts) == 1 {
				root = "(root files)"
			}

			// Avoid double counting the same directory for the same PR
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

	// Sort by Average Duration DESC
	sort.Slice(dirs, func(i, j int) bool {
		d1 := stats[dirs[i]]
		d2 := stats[dirs[j]]
		return (d1.TotalDuration / time.Duration(d1.Count)) > (d2.TotalDuration / time.Duration(d2.Count))
	})

	for i, d := range dirs {
		if i >= 5 {
			break
		} // Top 5
		s := stats[d]
		avg := s.TotalDuration / time.Duration(s.Count)
		fmt.Printf("   %-20s: %s (avg over %d PRs)\n", d, humanizeDuration(avg), s.Count)
	}
}

// NEW: Long Tail Authors
func printLongTailAuthors(prs []PullRequest) {
	fmt.Println("🐌 LONG TAIL CONTRIBUTORS (Handling the Slowest 10%)")

	// Sort by duration DESC
	sortedPRs := make([]PullRequest, len(prs))
	copy(sortedPRs, prs)
	sort.Slice(sortedPRs, func(i, j int) bool {
		return sortedPRs[i].MergedAt.Sub(sortedPRs[i].CreatedAt) > sortedPRs[j].MergedAt.Sub(sortedPRs[j].CreatedAt)
	})

	// Take top 10% slowest
	limit := len(prs) / 10
	if limit == 0 {
		limit = 1
	}
	slowest := sortedPRs[:limit]

	authorCounts := make(map[string]int)
	for _, pr := range slowest {
		authorCounts[pr.Author]++
	}

	// Sort authors by occurrences in the "slow bucket"
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

func printHistogram(prs []PullRequest) {
	fmt.Println("📊 MERGE TIME DISTRIBUTION")

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
