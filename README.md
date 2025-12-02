# Bottleneck

**Bottleneck** is a specialized CLI tool for Engineering Managers and Platform Engineers to analyze GitHub Pull Request velocity. It goes beyond simple averages to identify _where_ your development process is getting stuck.

## ✨ Features

- **📊 True Velocity Stats:** detailed breakdown of **Time to Merge** (Creation → Merge), including Median, Average, and Percentiles.
- **📐 Size vs Speed Analysis:** Calculates correlation between PR size (LOC) and speed. Tells you if you have a "Big PR" problem or a Process problem.
- **🔥 Directory Hotspots:** Identifies which parts of your codebase (e.g., `ios/`, `backend/`) are "swamps" that slow down the team.
- **🐌 Long Tail Contributors:** Highlights who is handling the most complex/slowest PRs (prevents burnout by making invisible work visible).
- **🚦 Review Efficiency:** Splits merge time into two critical phases:
    - **Triage Time:** (Created → First Review) - *Are PRs sitting invisible?*
    - **Review Time:** (First Review → Merged) - *Is the code too complex or CI too slow?*
- **📈 Monthly Trends:** visual indicators (🚀/🐢) to see if your team is speeding up or slowing down.
- **📉 Merge Distribution:** A histogram identifying the "long tail" of stuck PRs.
- **👥 Leaderboard:** Highlights the most active and fastest contributors.
- **✂️ Smart Filtering:** Options to exclude statistical outliers (top/bottom 5%) and fetch large datasets with automatic pagination.

## 🛠️ Prerequisites

- [Go](https://go.dev/) (1.21+ recommended)
- [GitHub CLI (`gh`)](https://cli.github.com/) installed and authenticated (`gh auth login`).
  > **Note:** This tool uses the `gh` CLI to fetch data securely. Ensure you have access to the target repositories.

## 🚀 Installation & Usage

### Option 1: Run directly (Development)

```bash
go run main.go [flags] <owner/repo>
```

### Option 2: Build and Install

```bash
go build -o bottleneck main.go
./bottleneck [flags] <owner/repo>
```

### Flags

- `--limit <n>`: Fetch the last `n` merged PRs (default 100). Supports pagination for large datasets (e.g., 1000+).
- `--exclude-outliers`: Automatically exclude the fastest and slowest 5% of PRs to remove noise (e.g., immediate self-merges or stale experiments).

### Example Command

```bash
go run main.go --limit 200 --exclude-outliers lancedb/lancedb
```

## 📋 Sample Output

```text
🔍 Fetching merged PRs for lancedb/lancedb (limit 200)...
✂️  Outlier filtering active. Reduced from 200 to 180 PRs.
------------------------------------------------------------
📊 GENERAL STATISTICS
   (Time from PR Creation -> Merge)
   Count:   180
   Average: 1d 20h
   Median:  9h 39m
   Min:     6m 45s
   Max:     17d 4h
------------------------------------------------------------
🚦 REVIEW EFFICIENCY
   Avg Time to First Review:   20h 15m (Triage Speed)
   Avg Review to Merge:        1d 0h (Coding/Fixing Speed)
   Reviewed PRs:               180/180

   💡 INSIGHT:
   • High 'Time to First Review'? -> You have a TRIAGE problem (nobody is looking).
   • High 'Review to Merge'?      -> You have a COMPLEXITY or TESTING problem.
------------------------------------------------------------
📈 MONTHLY TRENDS
   2025-10: 2d 3h           (29 PRs) 🐢
   2025-11: 1d 4h           (30 PRs) 🚀
   2025-12: 1d 15h          ( 5 PRs) 🐢
------------------------------------------------------------
```

## 🧠 Interpreting the Data

This tool is designed to help you answer specific questions:

1.  **"Why does it feel slow?"**
    - Check **General Statistics**. If the _Average_ is much higher than the _Median_, you have a few "nightmare PRs" dragging the team down, but the typical flow is fine.
2.  **"Are we ignoring PRs?"**
    - Check **Time to First Review**. If this is high (> 24h), your team needs better notifications or a "Reviewer of the Day" rotation.
3.  **"Is the code too hard to review?"**
    - Check **Review to Merge**. If this is high, PRs are likely too large, requirements are unclear, or your CI pipeline is flaky/slow.

## License

MIT

## By [@josephgoksu](https://x.com/josephgoksu)

[![Twitter Follow](https://img.shields.io/twitter/follow/josephgoksu)](https://x.com/josephgoksu)

[![Star History Chart](https://api.star-history.com/svg?repos=josephgoksu/bottleneck&type=Date)](https://www.star-history.com/#josephgoksu/bottleneck&Date)
