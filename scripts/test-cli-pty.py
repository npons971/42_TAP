#!/usr/bin/env python3
"""Linux integration check: idle server EOF, typed input, terminal restoration."""
import os
import pathlib
import pty
import select
import socket
import subprocess
import termios
import threading
import time

ROOT = pathlib.Path(__file__).resolve().parent.parent


def check(mode):
    listener = socket.socket()
    listener.bind(('127.0.0.1', 0))
    listener.listen()
    master, slave = pty.openpty()
    before = termios.tcgetattr(slave)
    commands = []
    failures = []

    def serve():
        try:
            conn, _ = listener.accept()
            with conn:
                conn.settimeout(3)
                conn.sendall(b'OK hello proto=1\n')
                if mode == 'disconnect':
                    time.sleep(0.15)
                    conn.sendall(b'EVT GLOBAL CHAT bob hello\n')
                    time.sleep(0.15)
                else:
                    with conn.makefile('rb') as reader:
                        while True:
                            raw = reader.readline()
                            if not raw:
                                break
                            line = raw.decode().rstrip('\n')
                            commands.append(line)
                            if line == 'QUIT':
                                break
                            conn.sendall(b'OK\n')
        except Exception as error:
            failures.append(error)

    worker = threading.Thread(target=serve, daemon=True)
    worker.start()
    proc = subprocess.Popen(
        [str(ROOT / '.build/tap-cli'), '-addr',
         '127.0.0.1:' + str(listener.getsockname()[1])],
        stdin=slave, stdout=slave, stderr=slave)
    try:
        transcript = b''
        deadline = time.monotonic() + 3
        while b'OK hello proto=1' not in transcript:
            if time.monotonic() > deadline:
                raise AssertionError('CLI greeting not displayed')
            ready, _, _ = select.select([master], [], [], .1)
            if ready:
                transcript += os.read(master, 65536)
        if mode == 'input':
            os.write(master, 'TALK Village Guard\rQUIT\r'.encode())
        elif mode == 'signal':
            proc.terminate()
        assert proc.wait(timeout=5) == 0
        assert termios.tcgetattr(slave) == before, 'terminal not restored'
        worker.join(timeout=4)
        assert not worker.is_alive(), 'mock server remained blocked'
        if mode == 'input':
            assert commands == ['TALK Village Guard', 'QUIT'], commands
        # Cancellation may close the peer while the mock attempts its final read.
        if mode != 'signal':
            assert not failures, failures
        print('CLI PTY passed:', mode)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait()
        listener.close()
        os.close(master)
        os.close(slave)


for scenario in ('disconnect', 'input', 'signal'):
    check(scenario)
