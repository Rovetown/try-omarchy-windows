import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('open-guest-lock-pr.sh')


class GuestLockPRTests(unittest.TestCase):
    def run_flow(self, reject, dispatch_fails=False):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            bindir = root / 'bin'
            bindir.mkdir()
            (bindir / 'git').write_text('#!/bin/sh\nexit 0\n')
            (bindir / 'gh').write_text('''#!/bin/sh
printf '%s\\n' "$*" >>"$CALLS"
if [ "$1" = workflow ] && [ "$DISPATCH_FAILS" = 1 ]; then exit 1; fi
if [ "$2" = create ]; then
  if [ "$REJECT" = 1 ]; then
    echo 'GitHub Actions is not permitted to create pull requests' >&2
    exit 1
  fi
  echo https://github.com/example/repo/pull/1
fi
''')
            for p in bindir.iterdir():
                p.chmod(0o755)
            env = dict(os.environ, PATH=f'{bindir}:{os.environ["PATH"]}',
                       GITHUB_REPOSITORY='example/repo', GITHUB_RUN_ID='123',
                       GITHUB_RUN_ATTEMPT='2', GITHUB_STEP_SUMMARY=str(root / 'summary'),
                       CALLS=str(root / 'calls'), REJECT=str(int(reject)), DISPATCH_FAILS=str(int(dispatch_fails)))
            result = subprocess.run(['bash', str(SCRIPT)], env=env, cwd=root,
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            return (root / 'summary').read_text(), (root / 'calls').read_text()

    def test_policy_denial_leaves_reviewable_branch(self):
        summary, calls = self.run_flow(True)
        self.assertIn('No PR was created', summary)
        self.assertIn('master...guest-lock/123-2', summary)
        self.assertNotIn('pr view', calls)

    def test_dispatch_failure_is_visible_without_losing_the_branch(self):
        summary, calls = self.run_flow(True, dispatch_fails=True)
        self.assertIn('CI was not started', summary)
        self.assertIn('master...guest-lock/123-2', summary)
        self.assertIn('pr create --draft', calls)

    def test_success_creates_draft_and_fetches_live_pr(self):
        summary, calls = self.run_flow(False)
        self.assertIn('pull/1', summary)
        self.assertIn('pr create --draft', calls)
        self.assertIn('workflow run ci.yml --ref guest-lock/123-2', calls)
        self.assertIn('pr view https://github.com/example/repo/pull/1', calls)


if __name__ == '__main__':
    unittest.main()
