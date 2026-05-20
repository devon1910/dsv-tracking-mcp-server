/*
Package browser provides a chromedp-backed headless Chromium singleton used
to fetch JSON endpoints that sit behind Cap.js (a combined proof-of-work and
browser-instrumentation captcha).

Cap.js combines proof-of-work with browser-instrumentation challenges. The
proof-of-work component is technically solvable in pure Go, but the
instrumentation layer (server-generated JS verifying DOM operations) is
explicitly designed to resist headless solvers without a real browser
environment. Per the Cap.js author's guidance, automated bypass is not the
intended path. This package therefore runs a real headless Chromium instance
so the captcha resolves through its designed mechanism. This approach is
slower at cold-start and uses more memory than a pure-HTTP client, but is
honest and reliable.

The Browser instance is a process-wide singleton that should be created at
server startup and closed during shutdown. It is safe to create many tabs
(chromedp contexts) concurrently; the browser process and cookie jar are
shared so once Cap.js validates in one tab further navigations within the
session benefit from the validated state until it expires.
*/
package browser
