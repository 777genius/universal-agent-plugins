#!/usr/bin/env python3
"""Recognize only the two observed failures of the pinned consent baseline."""
import json
from pathlib import Path
import re
import sys

PACKAGE = 'github.com/777genius/plugin-kit-ai/cli/internal/terminalprompts'
PARENT = 'TestDarwinVirtualTTYConsent'
TARGET = PARENT + '/queued-escape-enter-next-owner'


def classify(rows):
    if not rows or any(row.get('Package') != PACKAGE for row in rows):
        raise ValueError('unexpected baseline package')
    tests = {row.get('Test') for row in rows if row.get('Action') == 'run'}
    if tests != {PARENT, TARGET}:
        raise ValueError('exact consent baseline target must run alone')
    if rows[-1].get('Action') != 'fail' or rows[-1].get('Test'):
        raise ValueError('baseline package must fail')
    if any(row.get('Test') == TARGET and row.get('Action') in {'pass', 'skip'} for row in rows):
        raise ValueError('baseline target must not pass or skip')
    output = ''.join(row.get('Output', '') for row in rows if row.get('Test') == TARGET)
    if (any(row.get('Test') == TARGET and row.get('Action') == 'fail' for row in rows)
            and 'next owner: "" context deadline exceeded' in output):
        return 'next-owner-input-lost'

    # Go's alarm can abort the old parent before it emits a named test failure:
    # its 15s CommandContext cancellation is stuck reaping the virtual-TTY child.
    # A timeout alone proves nothing. Both goroutines and the exact running test
    # must match the captured macOS cancellation/reaping deadlock.
    goroutines = re.split(r'(?=^goroutine \d+ )', output, flags=re.M)
    wait_chain = any(
        re.match(r'goroutine \d+ \[syscall\]:', block)
        and re.search(r'syscall\.Wait4\(.*?os\.\(\*Process\)\.pidWait\(.*?'
                      r'os/exec\.\(\*Cmd\)\.Wait\(.*?'
                      r'terminalprompts\.TestDarwinVirtualTTYConsent\.func1\(.*?'
                      r'/consent_virtual_darwin_test\.go:\d+', block, re.S)
        for block in goroutines)
    cancelled_child = any(
        re.match(r'goroutine \d+ \[chan send\]:', block)
        and 'os/exec.(*Cmd).watchCtx(' in block
        and 'created by os/exec.(*Cmd).Start in goroutine ' in block
        for block in goroutines)
    if (not any(row.get('Test') == TARGET and row.get('Action') == 'fail' for row in rows)
            and 'panic: test timed out after 25s\n' in output
            and '\t\t' + TARGET + ' (25s)\n' in output
            and wait_chain and cancelled_child):
        return 'cancelled-virtual-tty-child-reap-deadlock'
    raise ValueError('unrecognized consent baseline failure')


if __name__ == '__main__':
    print(classify([json.loads(line) for line in Path(sys.argv[1]).read_text().splitlines()]))
