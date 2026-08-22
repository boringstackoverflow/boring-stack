// Copy buttons for any <pre class="terminal" data-copy="...">.
//
// Its own file rather than an inline IIFE on the home page, because the badge
// snippet on /showcase needs the same behaviour. Loading it as a separate
// <script> element also means a syntax error here can't take out the signup
// form wiring next to it.
//
// The analytics event name comes from data-copy-event, defaulting to
// install_copy. That default is load-bearing: install_copy is a step in the
// adoption funnel, so a button that is NOT an install step must set its own
// name (the showcase badge sets showcase_badge_copy) rather than inflating a
// funnel number.
(function () {
  if (typeof bsEvent !== 'function') { bsEvent = function () {}; }

  document.querySelectorAll('pre.terminal[data-copy]').forEach(function (pre) {
    var btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'copy-btn';
    btn.textContent = 'copy';
    btn.setAttribute('aria-label', 'copy to clipboard');
    btn.addEventListener('click', function () {
      navigator.clipboard.writeText(pre.dataset.copy).then(function () {
        btn.textContent = 'copied';
        btn.dataset.state = 'copied';
        setTimeout(function () { btn.textContent = 'copy'; delete btn.dataset.state; }, 1500);
        // Only after the clipboard write actually resolved, and never allowed to
        // interfere with the "copied" label the user is waiting to see.
        try {
          bsEvent(pre.getAttribute('data-copy-event') || 'install_copy', {
            label: pre.getAttribute('aria-label') || '',
          });
        } catch (e) {}
      }).catch(function () {
        btn.textContent = 'failed';
        setTimeout(function () { btn.textContent = 'copy'; }, 1500);
      });
    });
    pre.appendChild(btn);
  });
})();
