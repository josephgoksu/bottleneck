# Bottleneck

## 📖 Project Overview

**Bottleneck** is a specialized CLI tool for Engineering Managers and Platform Engineers to analyze GitHub Pull Request velocity. It goes beyond simple averages to identify _where_ your development process is getting stuck, providing actionable insights into your team's workflow and codebase health.

### ✨ Key Features

-   **📊 True Velocity Stats:** Detailed breakdown of **Time to Merge** (from PR Creation → Merge), including Median, Average, and Percentiles.
-   **📐 Size vs Speed Analysis:** Calculates the correlation between PR size (Lines of Code changed) and merge time. This helps determine if large PRs are genuinely slowing you down or if the bottleneck lies elsewhere.
-   **🔥 Directory Hotspots:** Identifies which parts of your codebase (e.g., `ios/`, `backend/`) are "swamps" associated with the slowest average merge times.
-   **🐌 Long Tail Contributors:** Highlights authors who are most frequently involved in the slowest 10% of PRs, helping to identify areas of complexity or potential burnout.
-   **🚦 Review Efficiency:** Splits merge time into two critical phases:
    -   **Triage Time:** (Created → First Review) - _Are PRs sitting unnoticed?_
    -   **Review Time:** (First Review → Merged) - _Is the code too complex, or is CI/CD too slow?_
-   **📈 Monthly Trends:** Visual indicators (🚀/🐢) to easily see if your team's velocity is improving or degrading month-over-month.
-   **🔮 Forecast:** Provides a moving average prediction for the next 30 days based on recent trends.
-   **📉 Merge Distribution:** A histogram visualizing the distribution of merge times, helping to identify the "long tail" of stuck PRs.
-   **👥 Leaderboard:** Highlights the most active and fastest contributors based on average merge time.
-   **✂️ Smart Filtering:** Options to exclude statistical outliers (top/bottom 5%) and fetch large datasets with automatic pagination for comprehensive analysis.

## 🚀 Installation Guide

### 🛠️ Prerequisites

-   [Go](https://go.dev/) (1.21+ recommended)
-   [GitHub CLI (`gh`)](https://cli.github.com/) installed and authenticated (`gh auth login`).
    > **Note:** This tool uses the `gh` CLI to fetch data securely. Ensure you have access to the target repositories you wish to analyze.

### Installation Steps

Choose one of the following methods:

#### Option 1: Run directly (for Development or quick use)

Clone the repository and run the `main.go` file directly.

```bash
git clone https://github.com/josephgoksu/bottleneck.git # Replace with actual repo link if different
cd bottleneck
go run main.go [flags] <owner/repo>
```

#### Option 2: Build and Install (recommended for regular use)

Build the executable and install it to your Go bin path, making it available as a global command.

```bash
go build -o bottleneck main.go
go install # This will place the 'bottleneck' executable in your $GOPATH/bin
bottleneck [flags] <owner/repo>
```

## 📖 Usage Instructions

To analyze a GitHub repository, provide the repository owner and name in the format `<owner>/<repo>`.

```bash
bottleneck [flags] <owner/repo>
```

### Flags

-   `--limit <n>`: Specifies the maximum number of merged PRs to fetch. The tool supports pagination for large datasets (e.g., 1000+ PRs). Default: `100`.
-   `--exclude-outliers`: When enabled, the fastest and slowest 5% of PRs are excluded from the analysis. This helps to remove noise from immediate self-merges or extremely stale experimental PRs. Default: `false`.
-   `--timeout <duration>`: Sets a timeout for each individual GitHub API request. If a request takes longer than this duration, it will be cancelled. Default: `30s`.
-   `--delay <duration>`: Sets a delay between sequential GitHub API requests. This helps in adhering to GitHub API rate limits. Default: `200ms`.

### Example Command

Analyze the `lancedb/lancedb` repository, fetching up to 200 PRs and excluding outliers:

```bash
bottleneck --limit 200 --exclude-outliers lancedb/lancedb
```

## ⚙️ Configuration

The `--timeout` and `--delay` flags provide configuration options to manage API interaction:

-   **`--timeout`**: Adjust this if you experience frequent request cancellations due to slow network conditions or large query responses. For example, `--timeout 60s`.
-   **`--delay`**: Increase this value (e.g., `--delay 500ms` or `--delay 1s`) if you encounter GitHub API rate limiting errors, especially when fetching a very high `--limit` of PRs.

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
------------------------------------------------------------
📐 SIZE vs SPEED ANALYSIS
   (Does the size of a PR affect how fast it merges?)
   Correlation Coeff: 0.11  (Range: -1.0 to +1.0)
   ✅ RESULT: Weak/No Correlation (< 0.3)
      Insight: Small PRs are getting stuck just as often as huge ones.
      Action:  Your bottleneck is likely PROCESS (Triage/CI/Availability), not code size.
------------------------------------------------------------
🔥 DIRECTORY HOTSPOTS (Avg Merge Time)
   nodejs              : 3d 21h (avg over 40 PRs)
   java                : 3d 14h (avg over 5 PRs)
   docs                : 2d 19h (avg over 20 PRs)
   python              : 2d 10h (avg over 70 PRs)
   rust                : 2d 4h (avg over 57 PRs)
------------------------------------------------------------
🐌 LONG TAIL CONTRIBUTORS (Handling the Slowest 10%)
   westonpace     : 3 slow PRs
   naaa760        : 3 slow PRs
   wjones127      : 2 slow PRs
   BubbleCal      : 2 slow PRs
   jackye1995     : 2 slow PRs
   (Note: These authors might be tackling the hardest complexity, not working slowly.)
------------------------------------------------------------
📈 MONTHLY TRENDS
   2025-10: 2d 3h           (29 PRs) 🐢
   2025-11: 1d 4h           (30 PRs) 🚀
   2025-12: 1d 15h          ( 5 PRs) 🐢
------------------------------------------------------------
🔮 FORECAST (Next 30 Days)
   Based on last 3 months:
   - 2025-10: 2d 3h
   - 2025-11: 1d 4h
   - 2025-12: 1d 15h

   🎯 PREDICTION: ~1d 15h / PR
   🏁 TREND:      📈 Speeding Up
------------------------------------------------------------
📊 MERGE TIME DISTRIBUTION
   < 1h       : ■■                   (15)
   1h - 1d    : ■■■■■■■■■■■■■■■■■■■■ (106)
   1d - 1w    : ■■■■■■■■             (44)
   1w - 1mo   : ■■                   (15)
   > 1mo      :                      (0)
```

## 🧠 Interpreting the Data

This tool is designed to help you answer specific questions by providing data-driven insights:

1.  **"Why does it feel slow?"**
    -   Check **General Statistics**. If the _Average_ merge time is significantly higher than the _Median_, you likely have a few "nightmare PRs" that are taking an exceptionally long time to merge. These outliers drag down the average, but your typical PR flow might be faster. Focus on identifying and resolving these long-stuck PRs.
2.  **"Are we ignoring PRs, or is review taking too long?"**
    -   Examine **Review Efficiency**.
        -   If **Avg Time to First Review** is high (e.g., consistently > 24 hours), your team might have a **TRIAGE** problem. PRs are sitting unnoticed in the queue. Consider implementing a "Reviewer of the Day" rotation or dedicated daily PR grooming sessions.
        -   If **Avg Review to Merge** is high, your team might have a **COMPLEXITY** or **TESTING** problem. PRs are being looked at, but they are difficult to understand, test, or approve. Encourage smaller, more focused PRs, pair programming, or invest in faster/more reliable CI/CD pipelines.
3.  **"Is the codebase itself causing bottlenecks?"**
    -   Look at **Directory Hotspots**. Directories with consistently high average merge times might indicate areas of the codebase that are inherently complex, tightly coupled, or frequently introduce bugs. This could suggest a need for refactoring, improved testing, or specialized domain expertise for reviews in those areas.
4.  **"Is PR size impacting our velocity?"**
    -   Consult **Size vs Speed Analysis**.
        -   A **Strong Positive Correlation** means larger PRs significantly increase merge time. Focus on breaking down work into smaller, more manageable PRs.
        -   A **Weak/No Correlation** suggests that PR size isn't the primary issue. Small PRs are getting stuck just as often as large ones, pointing towards process-related bottlenecks (triage, review, CI).

## 🤝 Contributing

Contributions are welcome! If you have suggestions for new features, improvements, or bug fixes, please open an issue or submit a pull request.

## 📄 License

MIT

## By [@josephgoksu](https://x.com/josephgoksu)

[![Twitter Follow](https://img.shields.io/twitter/follow/josephgoksu)](https://x.com/josephgoksu)

[![Star History Chart](https://api.star-history.com/svg?repos=josephgoksu/bottleneck&type=Date)](https://www.star-history.com/#josephgoksu/bottleneck&Date)