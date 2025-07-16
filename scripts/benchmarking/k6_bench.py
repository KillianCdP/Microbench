#!/usr/bin/env python3

import argparse
import os
import subprocess
import sys
import tempfile
from pathlib import Path

def main():
    parser = argparse.ArgumentParser(description='Run k6 benchmarks')
    parser.add_argument('-t', '--target', required=True, help='Target URL to benchmark')
    parser.add_argument('-r', '--rps', required=True, type=int, help='Requests per second')
    parser.add_argument('-b', '--bench-id', help='Benchmark ID')
    parser.add_argument('-R', '--replicas', type=int, help='Number of replicas')

    args = parser.parse_args()

    # Get k6 path
    k6_path = os.getenv('K6_PATH', 'k6')

    script_dir = Path(__file__).parent
    template_path = script_dir / 'script_template.js'

    if not template_path.exists():
        print(f"Error: Template not found: {template_path}")
        sys.exit(1)

    with open(template_path, 'r') as f:
        template_content = f.read()

    script_content = template_content.replace('{{RPS}}', str(args.rps))

    with tempfile.NamedTemporaryFile(mode='w', suffix='.js', delete=False) as f:
        f.write(script_content)
        script_path = f.name

    cmd = [k6_path, 'run', '-e', f'URL={args.target}', script_path]

    if args.bench_id:
        cmd.extend(['--tag', f'benchid={args.bench_id}'])

    if args.replicas:
        cmd.extend(['--tag', f'replicas={args.replicas}'])

    print(f"Running: {' '.join(cmd)}")

    # Execute
    subprocess.run(cmd)

if __name__ == '__main__':
    main()
