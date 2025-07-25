package main

// script to collect LCP time
const lcpScript = `(() => {
	window.__lcp = 0;
	
	new PerformanceObserver((list) => {
  		const entries = list.getEntries();
  		const lastEntry = entries[entries.length - 1]; // use latest LCP candidate
  		
		window.__lcp = lastEntry.startTime || 0;
	}).observe({ type: "largest-contentful-paint", buffered: true });
})();`

// script to capture console errors and warnings, and request errors
const errScript = `(() => {
	window.__console_errors = [];
	window.__request_errors = [];

	// capture resource and JS errors
	window.addEventListener('error', (e) => {
		if (e.target && (e.target.src || e.target.href)) {
			const message = (e.target.src || e.target.href) + " (type: " + e.target.tagName + ")";
			window.__request_errors.push("[Resource Load Failed]: " + message);
			return;
		}

		const message = e.message + " at " + e.filename + ":" + e.lineno + ":" + e.colno + " (" + e.error?.stack + ")";
		window.__console_errors.push("[Uncaught JS Error]: " + message);
	}, true);
	
	// capture unhandled promise rejections
	window.addEventListener('unhandledrejection', (e) => {
		const message = (e.reason ? e.reason.message : "Unknown") + " (" + e.reason?.stack + ")";
		window.__console_errors.push("[Unhandled Promise Rejection]: " + message);
	});
	
	// override fetch to capture request errors
	const origFetch = fetch;
	fetch = async function(...args) {
		try {
			const res = await origFetch.apply(this, args);
			
			if (res.status >= 400) {
				const message = res.status + " for " + res.url;
				window.__request_errors.push("[HTTP Error]: " + message);
			}
			
			return res;
		} catch (e) {
		 	const message = e.message + " for " + (args ? args[0] : "");
			window.__request_errors.push("[HTTP Error]: " + message);
			throw e;
		}
	};

	// override XMLHttpRequest to capture request errors
	const origOpen = XMLHttpRequest.prototype.open;
  	const origSend = XMLHttpRequest.prototype.send;
  	XMLHttpRequest.prototype.open = function (method, url, async, user, password) {
    	this.__requestUrl = url;
    	return origOpen.apply(this, arguments);
  	};
 	XMLHttpRequest.prototype.send = function (body) {
    	const xhr = this;

    	function logError() {
			if (xhr.status >= 400 || xhr.status === 0) {
				const message = xhr.status + " for " + xhr.__requestUrl;
				window.__request_errors.push("[HTTP Error]: " + message);
			}
    	}

		this.addEventListener("load", logError);
		this.addEventListener("error", logError);
		this.addEventListener("abort", logError);

		return origSend.apply(this, arguments);
	};

	// override console.error to capture console errors
	const originalConsoleError = console.error;
	console.error = (...args) => {
		const message = args.map(String).join(' ');
		window.__console_errors.push("[Error]: " + message);
		originalConsoleError.apply(console, args);
	};
	
	// override console.warn to capture console warnings
	const originalConsoleWarn = console.warn;
	console.warn = (...args) => {
		const message = args.map(String).join(' ');
		window.__console_errors.push("[Warning]: " + message);
		originalConsoleWarn.apply(console, args);
	};
	
	return window.__console_errors;
})();`

// script to collect mobile responsiveness issues
const responsiveScript = `(() => {
	const __responsiveIssues = [];
	let score = 100;

	// check for viewport meta tag
    const viewportTag = document.querySelector('meta[name="viewport"]');
	if (viewportTag) {
		const id = viewportTag.id;
		const isWix = id.includes('wix');
		if (!isWix) {
			const content = viewportTag.getAttribute('content') || '';
			const hasDeviceWidth = content.includes('width=device-width');
			if (!hasDeviceWidth) {
				__responsiveIssues.push("Viewport meta tag has an invalid width attribute");
				score -= 25;
			}
		}
	} else {
		__responsiveIssues.push("No viewport meta tag");
		score -= 30;
	}

	// check for media queries in stylesheets
	let hasMediaQueries = Array.from(document.styleSheets)
		.some(sheet => {
			try {
				return Array.from(sheet.cssRules).some(rule => rule.type === CSSRule.MEDIA_RULE);
			} catch(e) {
				// cross-origin stylesheet access error
				return false;
			}
		});
	if (!hasMediaQueries) {
		hasMediaQueries = Array.from(document.querySelectorAll('link[rel="stylesheet"]'))
			.some(link => {
				return link.media && link.media !== 'all' && link.media !== '';
			});
		if (!hasMediaQueries) {
			__responsiveIssues.push("No media queries in stylesheets");
			score -= 25;
		}
	}
	
	// check for horizontal scrollbar
	const horizontalBar = document.documentElement.scrollWidth > document.documentElement.clientWidth;
	if (horizontalBar) {
		__responsiveIssues.push("Has horizontal scrollbar");
		score -= 25;
	}

	// check for horizontally overflowing elements
    const overflowingElements = Array.from(document.querySelectorAll("*"))
        .filter(el => {
			if (el.offsetParent === null) return false; // skip invisible elements
			return el.scrollWidth > (el.clientWidth + 5);
		}).length;
    if (overflowingElements > 0) {
		__responsiveIssues.push("Has horizontally overflowing elements");
		score -= Math.min(15, overflowingElements * 2);
	}

	// check for small and crowded tap targets (links, buttons, etc.)
	const interactiveElements = Array.from(
		document.querySelectorAll('a, button, input, select, textarea, [onclick], [role="button"]')
	);
	const smallTapTargets = interactiveElements
		.filter(el => {
			if (el.offsetParent === null) return false; // skip invisible elements
			const rect = el.getBoundingClientRect();
			return (rect.width < 44 || rect.height < 44) && rect.width > 0 && rect.height > 0;
		}).length;
	if (smallTapTargets > 0) {
		__responsiveIssues.push("Has small tap targets");
		score -= Math.min(12, smallTapTargets * 1.2);
	}
	const crowdedTapTargets = interactiveElements
		.filter(el => {
			if (el.offsetParent === null) return false; // skip invisible elements
			const rect = el.getBoundingClientRect();
			const nearby = document.elementsFromPoint(rect.x + rect.width/2, rect.y + rect.height + 8);
			return nearby.some(n => 
				n !== el && 
				interactiveElements.includes(n) &&
				n.getBoundingClientRect().y < rect.y + rect.height + 16
			);
		}).length;
	if (crowdedTapTargets > 0) {
		__responsiveIssues.push("Has crowded tap targets");
		score -= Math.min(6, crowdedTapTargets * 0.6);
	}

	// check for non responsive images (wider than viewport)
	const inflexibleImages = Array.from(document.querySelectorAll('img'))
		.filter(img => {
			if (img.offsetParent === null) return false; // skip invisible images
			const style = window.getComputedStyle(img);
			const rect = img.getBoundingClientRect();
			return rect.width > window.innerWidth && 
				style.maxWidth === 'none' && !style.width.includes('%');
		}).length;
	if (inflexibleImages > 0) {
		__responsiveIssues.push("Has non flexible images");
		score -= Math.min(9, inflexibleImages * 1.8);
	}

	// check for small text
	const smallText = Array.from(
			document.querySelectorAll('p, h1, h2, h3, h4, h5, h6, span, a, li, td, th')
		).filter(el => {
			if (el.offsetParent === null || !el.textContent.trim()) return false; // skip invisible elements
			const style = window.getComputedStyle(el);
			const fontSize = parseFloat(style.fontSize);
            return fontSize < 12;
		}).length;
	if (smallText > 0) {
		__responsiveIssues.push("Has small text");
		score -= Math.min(9, smallText * 1.2);
	}

	// check for flexible layout
	const hasFlexLayout = Array.from(document.querySelectorAll(
			'main, .container, .wrapper, header, nav, section, article, aside, footer'
		)).some(el => {
			const style = window.getComputedStyle(el);
			return style.display.includes('flex') || 
				style.display.includes('grid') ||
				style.display === 'block' && (
					style.maxWidth.includes('%') || 
					style.width.includes('%') ||
					style.width === 'auto'
				);
		});
	if (!hasFlexLayout) {
		__responsiveIssues.push("No flexible layout patterns");
		score -= 10;
	}

	// ensure score doesn't go below 0
	const finalScore = Math.max(0, Math.round(score));
	const scoreType = (finalScore >= 75) ? '(Good ✅)' : (finalScore >= 60) ? '(Minor ⚠️)' : 
		(finalScore >= 45) ? '(Major 🛑)' : '(Critical ❌)';
	__responsiveIssues.push("Score: " + finalScore + " " + scoreType);

	return __responsiveIssues;
})()`

// script to collect form issues
const formValidationScript = `(() => {
    const __formIssues = [];
    
    // iterate over all forms in the document
    document.querySelectorAll('form').forEach((form, formIndex) => {
        const formSelector = form.id ? 
            'form#' + form.id : 
            'form:nth-of-type(' + (formIndex + 1) + ')';
        
        // check for presence of form submit handler
        const formAction = form.getAttribute('action') || form.getAttribute('onsubmit');
		const hasJsAttr = (form.hasAttribute('data-action') || form.hasAttribute('ng-submit') || 
			form.hasAttribute('v-on:submit') || form.hasAttribute('@submit'));
		const hasHtmxAttr = (form.hasAttribute("hx-get") || form.hasAttribute("hx-post") || 
			form.hasAttribute("hx-put") || form.hasAttribute("hx-patch") || form.hasAttribute("hx-delete"));
        if (!formAction && !hasJsAttr && !hasHtmxAttr) {
            __formIssues.push(formSelector + " is missing action attribute or JavaScript submit handler");
        }
        
		// check if form has a submit button
        const hasSubmitButton = !!form.querySelector('button[type="submit"], input[type="submit"], button:not([type])');
        if (!hasSubmitButton) {
			__formIssues.push(formSelector + " is missing a submit button");
        }

		// check for proper enctype for file uploads
        const hasFileInput = !!form.querySelector('input[type="file"]');
		if (hasFileInput && form.getAttribute('enctype') !== 'multipart/form-data') {
			__formIssues.push(formSelector + " is missing proper enctype='multipart/form-data'");
		}
        
        // iterate over all input elements excluding hidden and submit types
        form.
			querySelectorAll('input:not([type="hidden"]):not([type="submit"]):not([type="button"]), select, textarea').
			forEach((input, inputIndex) => {
				const tag = input.tagName.toLowerCase()
				const inputSelector = input.id ? tag + '#' + input.id : 
					input.name ? 
						tag + '[name="' + input.name + '"]' : 
						tag + ':nth-of-type(' + (inputIndex + 1) + ')';
				
				// check for presence of label or placeholder
				const hasLabel = input.id ? 
					!!document.querySelector('label[for="' + input.id + '"]') : 
					input.closest('label') !== null;
				const hasAriaLabel = input.hasAttribute('aria-label') && 
					input.getAttribute('aria-label').trim() !== '';
				const hasPlaceholder = input.hasAttribute('placeholder') && 
					input.getAttribute('placeholder').trim() !== '';
				if (!hasLabel && !hasAriaLabel && !hasPlaceholder) {
					__formIssues.push(inputSelector + " (in " + formSelector + ") is missing a label");
				}
				
				// check for presence of name attribute (crucial for form submission)
				if (!input.name && input.type !== 'button' && input.type !== 'submit') {
					__formIssues.push(inputSelector + " (in " + formSelector + ") is missing a name attribute");
				}

				// check for correct input type
				if (input.type === 'text' && (input.name || input.id)) {
					const name = (input.name || input.id || input.placeholder || '').toLowerCase();
					if (name.includes('email') && input.type !== 'email') {
						__formIssues.push(inputSelector + " (in " + formSelector + ") has incorrect type");
					}
					if ((name.includes('tel') || name.includes('phone')) && input.type !== 'tel') {
						__formIssues.push(inputSelector + " (in " + formSelector + ") has incorrect type");
					}
				}
				
				// check for passwords served over HTTP
				if (input.type === 'password') {
					if (window.location.protocol !== 'https:') {
						__formIssues.push(
							inputSelector + " (in " + formSelector + ") is a password field not served over HTTPS"
						);
					}
				}
        	});
    });
    
    return __formIssues;
})();`

// script to detect frontend technologies
const techScript = `(() => {
	const __detectedTech = [];

	const checks = {
    	'WordPress': () => {
			return document.body.innerHTML.includes('wp-content') || 
				window.wp || 
				document.querySelector('link[href*="wp-content"], link[href*="wp-includes"]') ||
				document.querySelector('meta[name="generator"][content*="WordPress"]') ||
				document.querySelector('link[rel="https://api.w.org/"]') ||
				document.body.classList.contains('wordpress') ||
				document.documentElement.innerHTML.includes('wp-json');
		},
		'Wix': () => {
			return document.body.innerHTML.includes('wixstatic') || 
				window.wixBiSession || 
				document.querySelector('[data-wix-id]') ||
				window.wixDevelopersAnalytics ||
				document.querySelector('meta[name="generator"][content*="Wix"]') ||
				document.documentElement.innerHTML.includes('wix.com');
		},
		'Webflow': () => {
			return document.querySelector('[data-wf-page]') || 
				window.Webflow || 
				document.querySelector('script[src*="webflow"]') ||
				document.querySelector('[data-wf-site]') ||
				document.querySelector('link[href*="webflow.css"]') ||
				document.documentElement.innerHTML.includes('webflow');
		},
		'Squarespace': () => {
			return document.body.innerHTML.includes('squarespace') || 
				document.body.id.includes('squarespace') || 
				window.Y ||
				document.querySelector('meta[name="generator"][content*="Squarespace"]') ||
				document.querySelector('body[id*="squarespace"]') ||
				document.querySelector('script[src*="squarespace"]');
		},
		'Shopify': () => {
			return document.body.innerHTML.includes('shopify') || 
				window.Shopify || 
				window.ShopifyAnalytics ||
				document.querySelector('input[name="form_type"][value*="shopify"]') ||
				document.querySelector('meta[name="generator"][content*="Shopify"]') ||
				document.documentElement.innerHTML.includes('shopify-section');
		},
		'React': () => {
			return window.React || 
				document.querySelector('[data-reactroot], [data-react-helmet]') ||
				document.querySelector('script[src*="react"]') ||
				document.querySelector('[data-react-checksum]') ||
				(document.documentElement.innerHTML.includes('react') && 
				(document.querySelector('[class*="react"], [id*="react"]') || 
				document.querySelector('script').textContent.includes('React'))) ||
				Array.from(document.querySelectorAll('*')).some(el => el.hasAttribute && 
					Array.from(el.attributes).some(attr => attr.name.includes('data-react')));
		},
		'Vue': () => {
			return window.Vue || 
				window.__VUE__ ||
				document.querySelector('script[src*="vue"]') ||
				document.querySelector('[data-v-app]') ||
				document.querySelector('[v-cloak]') ||
				Array.from(document.querySelectorAll('*')).some(el => 
					Array.from(el.attributes || []).some(attr => attr.name.startsWith('data-v-'))) ||
				document.documentElement.innerHTML.includes('data-v-');
		},
		'Angular': () => {
			return window.angular ||
				window.ng ||
				document.querySelector('[ng-version], [ng-app], app-root') ||
				document.querySelector('script[src*="angular"]') ||
				Array.from(document.querySelectorAll('*')).some(el => 
					Array.from(el.attributes || []).some(attr => attr.name.startsWith('ng-'))) ||
				document.documentElement.innerHTML.includes('ng-version') ||
				document.querySelector('[ng-controller]');
		},
		'Svelte': () => {
			return document.querySelector('[class*="svelte-"]') ||
				Array.from(document.querySelectorAll('*')).some(el => 
					Array.from(el.classList || []).some(cls => cls.includes('svelte-'))) ||
				document.querySelector('script[src*="svelte"]') ||
				document.documentElement.innerHTML.includes('svelte-');
		},
		'Solid.js': () => {
			return window.solid || 
				window.SolidJS ||
				document.querySelector('[data-solid]') ||
				document.querySelector('script[src*="solid"]') ||
				document.documentElement.innerHTML.includes('solid-js') ||
				Array.from(document.querySelectorAll('*')).some(el => 
					Array.from(el.attributes || []).some(attr => attr.name.includes('solid')));
		},
		'Next': () => {
			return document.querySelector('#__next') || 
				window.__NEXT_DATA__ || 
				document.querySelector('script[src*="_next"]') ||
				document.querySelector('link[href*="_next"]') ||
				document.querySelector('meta[name="generator"][content*="Next.js"]') ||
				document.documentElement.innerHTML.includes('__NEXT_DATA__');
		},
		'Nuxt': () => {
			return document.querySelector('#__nuxt') || 
				window.__NUXT__ || 
				document.querySelector('script[src*="_nuxt"]') ||
				document.querySelector('link[href*="_nuxt"]') ||
				document.querySelector('meta[name="generator"][content*="Nuxt.js"]') ||
				document.documentElement.innerHTML.includes('__NUXT__');
		},
		'Remix': () => {
			return window.__remixManifest || window.__remixContext ||
				document.querySelector('[data-remix-root]') ||
				document.querySelector('script[src*="remix"]') ||
				document.documentElement.innerHTML.includes('__remixManifest') ||
				document.querySelector('#remix-app') ||
				document.querySelector('link[rel="modulepreload"][href*="remix"]');
		},
		'HTMX': () => {
			return window.htmx ||
				document.querySelector('[hx-get], [hx-post], [hx-put], [hx-delete], [hx-patch]') ||
				document.querySelector('script[src*="htmx"]') ||
				Array.from(document.querySelectorAll('*')).some(el => 
					Array.from(el.attributes || []).some(attr => attr.name.startsWith('hx-'))) ||
				document.documentElement.innerHTML.includes('htmx') ||
				document.querySelector('[hx-trigger], [hx-target]');
		},
		'Alpine.js': () => {
			return window.Alpine ||
				document.querySelector('[x-data], [x-show], [x-if], [x-for]') ||
				document.querySelector('script[src*="alpine"]') ||
				Array.from(document.querySelectorAll('*')).some(el => 
					Array.from(el.attributes || []).some(attr => attr.name.startsWith('x-'))) ||
				document.documentElement.innerHTML.includes('alpine') ||
				document.querySelector('[x-text], [x-html], [x-model]');
		},
		'jQuery': () => {
			return window.jQuery || 
				(window.$ && window.$.fn && window.$.fn.jquery) ||
				document.querySelector('script[src*="jquery"]') ||
				(window.$ && typeof window.$.fn === 'object' && window.$.fn.constructor.toString().includes('jQuery'));
		},
		'Bootstrap': () => {
			return document.querySelector('link[href*="bootstrap"]') || 
				document.querySelector('script[src*="bootstrap"]') ||
				window.bootstrap || 
				((document.querySelector('.container, .row, .col') ||
				document.querySelector('.btn-primary, .btn-secondary, .btn-success') ||
				document.querySelector('.navbar-nav, .navbar-brand') ||
				document.querySelector('.modal-dialog, .modal-content') ||
				document.querySelector('.card-body, .card-header')) && 
				document.documentElement.innerHTML.includes('bootstrap'));
		},
		'Tailwind': () => {
			const specificTailwindClasses = [
				'bg-blue-', 'text-gray-', 'p-4', 'm-4', 'w-full', 'h-screen',
				'space-x-', 'divide-y', 'border-gray-', 'rounded-lg', 'shadow-lg'
			];
			const hasSpecificClasses = specificTailwindClasses.some(cls => 
				document.querySelector('[class*="' + cls + '"]'));
			const hasTailwindLink = document.querySelector('link[href*="tailwind"]') || 
				document.documentElement.innerHTML.includes('tailwindcss');
			const hasUtilityPattern = Array.from(document.querySelectorAll('*')).some(el => {
				const utilityCount = Array.from(el.classList || []).filter(cls => 
					cls.match(/^(bg|text|p|m|flex|grid|w|h|space|divide|border|rounded|shadow)-/)).length;
				return utilityCount >= 3; // at least 3 utility classes on one element
			});
			return hasTailwindLink || (hasSpecificClasses && hasUtilityPattern);
		},
  	};
  
	for (const [name, check] of Object.entries(checks)) {
		if (check()) {
			__detectedTech.push(name);
		}
	}

	return __detectedTech;
})();`
