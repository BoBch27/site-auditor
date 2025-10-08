# Site Auditor

A simple command-line tool written in Go that scans and audits multiple websites for common front-end issues including:

- Slow loading times
- JavaScript console errors
- Broken or missing assets (images, scripts, stylesheets)
- Visual layout bugs (e.g. overflows)
- Mobile responsiveness issues
- Broken forms or buttons
- Front end technologies used

## Features

✅ Bulk website scanning from a CSV list  
✅ Fetching websites from Google Places  
✅ Scraping websites from Google Search prompts  
✅ Outputs results to a new CSV file  
✅ Headless Chrome inspection using `chromedp`  
✅ Detects runtime JS errors and layout overflows  
✅ Enable and disable different checks  
✅ Run only critical/important checks  
✅ Full page screenshots  
✅ Easily extendable

## Installation

```bash
git clone https://github.com/BoBch27/site-auditor.git
cd site-auditor
go build -o site-auditor
```

Make sure you have Google Chrome or Chromium installed and accessible in your `PATH`.

## Usage

```bash
./site-auditor -input=websites.csv -output=results.csv
```
```bash
# set Maps API Key as an environment variable before running a Google Places Search
export MAPS_API_KEY="Your Maps API Key" && \
./site-auditor -search="beauty salon in manchester" -output=results.csv -checks=mobile,form,tech
```
```bash
./site-auditor -scrape="accountants liverpool" -output=results.csv -important
```

-`input`: Path to the input CSV file (must have a URL column)  
-`search`: Search prompt for which to find URLs from Google Places  
-`scrape`: Google input prompt to scrape URLs for  
-`output`: Path to the output CSV file to write results  
-`checks`: Comma-separated checks to run (security,lcp,console,request,headers,mobile,form,tech,screenshot). Empty = all checks  
-`important`: Run only critical/important checks (faster)
-`screenshot-dir`: Path to folder to store screenshots (if enabled)

## Example CSV Input

```csv
url
https://example.com
https://another-website.co.uk
```

Built with ❤️ in Go to help freelancers and developers offer better website audits.