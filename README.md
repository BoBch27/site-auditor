# Site Auditor

A simple command-line tool written in Go that scans and audits multiple websites for common front-end issues including:

- Slow loading times
- JavaScript console errors
- Broken or missing assets (images, scripts, stylesheets)
- Visual layout bugs (e.g. overflows)
- Mobile responsiveness issues
- Broken forms or buttons

## Features

✅ Bulk website scanning from a CSV list  
✅ Outputs results to a new CSV file  
✅ Headless Chrome inspection using `chromedp`  
✅ Detects runtime JS errors and layout overflows  
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

-`input`: Path to the input CSV file (must have a url column)  
-`output`: Path to the output CSV file to write results

## Example CSV Input

```csv
url
https://example.com
https://another-website.co.uk
```

Built with ❤️ in Go to help freelancers and developers offer better website audits.