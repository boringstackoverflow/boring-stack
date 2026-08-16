// Boring Stack funnel analytics.
//
// Sends anonymous page views and CTA clicks to api.boringstack.org, which is
// the canonical store for the funnel dashboard. No cookies, no localStorage, no
// third-party vendor. Visitors are counted server-side via a hash of IP + user
// agent that rotates monthly, so nothing here identifies anyone across months.
// Design notes live in the boring-stack-backend repo, decisions D18-D21.
//
// TWO RULES, both load-bearing:
//
//  1. This file is loaded as its OWN <script> element, never merged into a
//     page's inline script. Script elements have independent error boundaries:
//     an uncaught error in this file cannot stop a later inline block from
//     running. That is what keeps a broken beacon from taking down the
//     newsletter signup form on the home page.
//
//  2. Bodies are form-encoded (URLSearchParams). sendBeacon then sends
//     Content-Type: application/x-www-form-urlencoded, which is CORS-safelisted
//     and issues NO preflight. Switching to JSON would add a blocking OPTIONS
//     round-trip to every page view.

var bsFirstTouch, bsEvent;

(function () {
  var ENDPOINT = 'https://api.boringstack.org/v1/events';
  var KEY = 'bs.first_touch';

  function rand(n) {
    return (Math.random().toString(36).slice(2) + Date.now().toString(36)).slice(0, n);
  }

  // Returns {id, src, medium, campaign, ref} for this session, creating it on
  // first call. sessionStorage rather than localStorage: session scope is
  // enough to attribute a signup to the visit that produced it, without leaving
  // a persistent identifier behind. Storage access throws outright in some
  // privacy modes, so every path is guarded and degrades to {}.
  //
  // First-touch means first: once set for a session it is never rewritten, so a
  // visitor who arrives from Hacker News and later reloads with a campaign tag
  // still counts as Hacker News.
  bsFirstTouch = function () {
    try {
      var saved = sessionStorage.getItem(KEY);
      if (saved) return JSON.parse(saved);

      var qs = new URLSearchParams(location.search);
      var data = {
        id: rand(24),
        src: qs.get('utm_source') || '',
        medium: qs.get('utm_medium') || '',
        campaign: qs.get('utm_campaign') || '',
        ref: document.referrer || ''
      };
      sessionStorage.setItem(KEY, JSON.stringify(data));
      return data;
    } catch (e) {
      return {};
    }
  };

  // Fire and forget. Never throws, never returns anything, never blocks.
  bsEvent = function (name, props) {
    try {
      if (!navigator.sendBeacon && !window.fetch) return;
      var ft = bsFirstTouch();
      var body = new URLSearchParams();
      body.set('event', name);
      body.set('dedupe_key', rand(32));
      body.set('path', location.pathname);
      if (ft.id) body.set('attribution_id', ft.id);
      if (ft.src) body.set('utm_source', ft.src);
      if (ft.medium) body.set('utm_medium', ft.medium);
      if (ft.campaign) body.set('utm_campaign', ft.campaign);
      if (ft.ref) body.set('ref', ft.ref);
      for (var k in props) { if (props[k]) body.set(k, props[k]); }

      if (navigator.sendBeacon) {
        navigator.sendBeacon(ENDPOINT, body);
      } else {
        fetch(ENDPOINT, { method: 'POST', mode: 'no-cors', keepalive: true, body: body });
      }
    } catch (e) {
      // An event is never worth an exception.
    }
  };

  try {
    bsEvent('landing_page_view');

    // sendBeacon survives page unload, so outbound navigation is never delayed
    // and the link is never preventDefault'ed.
    document.addEventListener('click', function (ev) {
      try {
        var a = ev.target && ev.target.closest && ev.target.closest('a[href*="github.com"]');
        if (a) bsEvent('github_click', { target: a.getAttribute('href') || '' });
      } catch (e) {}
    }, true);
  } catch (e) {
    // Analytics is optional; the page is not.
  }
})();
