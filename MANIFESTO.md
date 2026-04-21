# The Boring Stack Manifesto

Seven principles for shipping software that stays out of your way.

---

## 1. Boring is a feature, not a fallback.

Mature tools have fewer surprises at 2am. They've shipped through enough corner cases that the bug you'd hit was hit, reported, and fixed five years ago. The thrill of the new is mostly procrastination wearing a hoodie.

Pick the thing your future self at 3am will be grateful for, not the thing your current self at 11pm finds exciting. They're rarely the same.

## 2. Your cloud bill is the metric.

Architecture decisions that don't show up on the bill at 100 customers are theoretical. The ones that do show up are usually the ones you wish you'd made differently.

The boring stack puts your monthly cost low enough that a profitable side project costs less than a coffee subscription. Below that threshold, you can afford to keep shipping until something works. Above it, every month is pressure.

## 3. SQLite is a database. Stop apologizing.

In WAL mode, on a $5 VPS, SQLite handles more writes per second than most apps will ever see. Litestream replicates it continuously to S3-compatible storage for under a dollar a month. You can restore to any second in the past 24 hours.

The "what if I outgrow it" worry is mostly a tax you pay forever to avoid a problem you may never have. When you do hit the wall, `pgloader` migrates the file in an afternoon.

## 4. If it requires Kubernetes, you don't have a web app, you have a server farm.

Kubernetes is correct for the company that needed it. For the side project, it's a hobby on top of a hobby, with orchestration overhead larger than the thing being orchestrated.

systemd is already on every Linux box you'll ever rent. `Restart=on-failure` is one line. `journalctl` is your log aggregator. The complexity you skip is complexity you don't have to maintain at 2am.

## 5. Your deploy script should fit on a postcard.

Build. scp. systemctl restart. curl healthz. That's a deploy.

If yours is more than that, ask what each extra line buys you. Most extra lines buy speed at traffic levels you don't have, or safety against failure modes that almost never fire on a single-server app. The simpler script costs you nothing and you can read it without coffee.

## 6. One person should be able to fully understand the running system.

Complexity that exceeds one head is complexity you pay for forever. In onboarding, in debugging, in the latency between "we should do X" and someone confident enough to do X.

The test: if you got hit by a bus tomorrow, could a competent stranger read the repo on Saturday and ship a fix on Sunday? If no, simplify until yes.

## 7. The babysitter is more interesting than the cleverness.

Boring tech needs operational discipline: backups verified, alerts when the cron silently stops, restore drills, kill switches. That discipline IS the work. The stack is just the canvas.

You don't earn the right to be clever in your code by being clever in your code. You earn it by making sure the boring parts work, every day, without you watching.

---

## How to use this manifesto

Quote it. Link to it. Print it on a postcard. Argue with it. Fork it and write your own.

Maintained by [boringstackoverflow](https://github.com/boringstackoverflow) as part of the Boring Stack project.
