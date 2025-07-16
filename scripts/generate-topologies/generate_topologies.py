#!/usr/bin/env python3

import argparse
import os
from concurrent.futures import ProcessPoolExecutor, as_completed
from typing import List, Dict, Any


def generate_yaml(depth: int, services_per_level: List[int], replicas_per_node: int,
                  nodes: List[str], scheduling: str = 'rr') -> str:
    """
    Generate a YAML topology configuration.
    """
    services = {}

    for level in range(depth):
        num_services = services_per_level[level]

        if scheduling == 'hh':
            node = nodes[0] if level < depth / 2 else nodes[-1]
        elif scheduling == 'rr':
            node = nodes[level % len(nodes)]
        else: # Either scheduling is "same" or there is an error, so still using "same" just in case
            node = nodes[0]

        for service_num in range(num_services):
            if level == 0:
                service_name = "frontend"
            else:
                service_name = f"service-d{level}-{service_num + 1}"

            out_services = []
            if level < depth - 1:
                next_level_services = services_per_level[level + 1]
                for next_service_num in range(next_level_services):
                    out_services.append(f"service-d{level+1}-{next_service_num+1}")

            services[service_name] = {
                'node': node,
                'out_services': out_services,
                'port': 50051,
                'processing_delay': '0ms',
                'replicas': replicas_per_node
            }

    # Generate YAML output manually to avoid dependency
    yaml_lines = ['services:']
    for name, service in services.items():
        yaml_lines.append(f'  {name}:')
        yaml_lines.append(f'    node: {service["node"]}')
        yaml_lines.append(f'    port: {service["port"]}')
        yaml_lines.append(f'    processing_delay: {service["processing_delay"]}')
        yaml_lines.append(f'    replicas: {service["replicas"]}')
        yaml_lines.append(f'    out_services: {service["out_services"]}')
    
    return '\n'.join(yaml_lines)


def generate_single_topology(params: Dict[str, Any]) -> Dict[str, str]:
    """Generate a single topology file with given parameters."""
    depth, services_str, replicas, nodes, scheduling, filename = params

    services_per_level = list(map(int, services_str.split(',')))
    nodes_list = nodes.split(',')

    yaml_content = generate_yaml(depth, services_per_level, replicas, nodes_list, scheduling)

    return {
        'filename': filename,
        'content': yaml_content
    }


def generate_service_chain(depth: int) -> str:
    """Generate service chain string from depth (e.g., depth=4 -> "1,1,1,1")."""
    return ','.join(['1'] * depth)

def generate_bulk_topologies(output_dir: str, nodes: str = "sdn2,sdn4",
                           replicas: int = 8, workers: int = 4):
    """
    Generate bulk topologies
    This is what we used to generate the topologies for our experiments.
    """
    os.makedirs(output_dir, exist_ok=True)

    tasks = []

    # 1. Depth variations (Chain topologies)
    print("=== Generating Chain Topologies ===")
    for depth in [2, 4, 6, 8, 10]:
        services = generate_service_chain(depth)
        for scheduling in ['hh', 'rr', 'same']:
            filename = f"depth_{depth}_{scheduling}.yaml"
            tasks.append((depth, services, replicas, nodes, scheduling, filename))

    # 2. Fan-In / Fan-Out variations
    print("=== Generating Fan Topologies ===")
    for n in [1, 2, 4, 6, 8]:
        for i in [1, 2, 4, 6, 8]:
            for m in [1, 2, 4, 6, 8]:
                services = f"1,{n},{i},{m}"
                for scheduling in ['hh', 'rr', 'same']:
                    filename = f"fan_{n}_{i}_{m}_{scheduling}.yaml"
                    tasks.append((4, services, replicas, nodes, scheduling, filename))

    # 3. Diamond topologies
    print("=== Generating Diamond Topologies ===")
    diamond_services = [
        "1,2,4,2", "1,2,5,2", "1,2,6,2", "1,2,7,2", "1,2,8,2",
        "1,3,5,3", "1,3,6,3", "1,3,7,3", "1,3,8,3"
    ]

    for services in diamond_services:
        parts = services.split(',')[1:]  # Skip first '1'
        parts_str = '_'.join(parts)
        for scheduling in ['same', 'rr']:
            filename = f"diamond_{parts_str}_{scheduling}.yaml"
            tasks.append((4, services, 1, nodes, scheduling, filename))

    # 4. Butterfly topologies
    print("=== Generating Butterfly Topologies ===")
    butterfly_services = [
        "1,3,1,3", "1,4,1,4", "1,4,2,4", "1,5,1,5", "1,5,2,5",
        "1,5,3,5", "1,6,1,6", "1,6,2,6", "1,7,1,7", "1,8,1,8"
    ]

    for services in butterfly_services:
        parts = services.split(',')[1:]  # Skip first '1'
        parts_str = '_'.join(parts)
        for scheduling in ['same', 'rr']:
            filename = f"butterfly_{parts_str}_{scheduling}.yaml"
            tasks.append((4, services, 1, nodes, scheduling, filename))

    # 5. Funnel topologies
    print("=== Generating Funnel Topologies ===")
    funnel_services = [
        "1,3,3,1", "1,4,4,1", "1,4,4,2", "1,5,5,1", "1,5,5,2", "1,6,6,1"
    ]

    for services in funnel_services:
        parts = services.split(',')[1:]  # Skip first '1'
        parts_str = '_'.join(parts)
        for scheduling in ['same', 'rr']:
            filename = f"funnel_{parts_str}_{scheduling}.yaml"
            tasks.append((4, services, 1, nodes, scheduling, filename))

    # 6. Reverse funnel topologies
    print("=== Generating Reverse Funnel Topologies ===")
    reverse_funnel_services = [
        "1,1,3,3", "1,1,4,4", "1,2,4,4", "1,1,5,5", "1,2,5,5",
        "1,3,5,5", "1,1,6,6", "1,2,6,6", "1,1,7,7", "1,1,8,8"
    ]

    for services in reverse_funnel_services:
        parts = services.split(',')[1:]  # Skip first '1'
        parts_str = '_'.join(parts)
        for scheduling in ['same', 'rr']:
            filename = f"rev_funnel_{parts_str}_{scheduling}.yaml"
            tasks.append((4, services, 1, nodes, scheduling, filename))

    print(f"\nGenerating {len(tasks)} topology files with {workers} workers...")

    with ProcessPoolExecutor(max_workers=workers) as executor:
        future_to_task = {executor.submit(generate_single_topology, task): task for task in tasks}

        completed = 0
        for future in as_completed(future_to_task):
            result = future.result()

            filepath = os.path.join(output_dir, result['filename'])
            with open(filepath, 'w') as f:
                f.write(result['content'])

            completed += 1
            if completed % 50 == 0: # Don't print everything
                print(f"Generated {completed}/{len(tasks)} files...")

    print(f"\nCompleted! Generated {len(tasks)} topology files in {output_dir}")

    categories = ['depth', 'fan', 'diamond', 'butterfly', 'funnel', 'rev_funnel']
    for category in categories:
        count = len([f for f in os.listdir(output_dir) if f.startswith(category)])
        print(f"  {category}: {count} files")


def main():
    parser = argparse.ArgumentParser(
        description="Generate YAML topology configurations for MicroBench",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Generate a single chain topology
  python generate_topologies.py --depth 4 --services "1,1,1,1" --replicas 2 --nodes "worker1,worker2" --scheduling rr

  # Generate a fan topology
  python generate_topologies.py --depth 3 --services "1,3,1" --replicas 1 --nodes "node1,node2" --scheduling same

  # Generate bulk topologies for CNI testing
  python generate_topologies.py --bulk --output-dir ./topologies --nodes "sdn2,sdn4" --replicas 8 --workers 8
        """
    )

    # Single topology generation
    parser.add_argument('--depth', type=int, help='Maximum call depth')
    parser.add_argument('--services', type=str, help='Comma-separated list of services per level')
    parser.add_argument('--replicas', type=int, default=4, help='Number of replicas per service')
    parser.add_argument('--nodes', type=str, required=True, help='Comma-separated list of nodes')
    parser.add_argument('--scheduling', type=str, default='rr',
                        choices=['hh', 'rr', 'same'],
                        help='Scheduling strategy (hh=half-half, rr=round-robin, same=all on one node)')

    # Bulk generation
    parser.add_argument('--bulk', action='store_true', help='Generate bulk topologies')
    parser.add_argument('--output-dir', type=str, default='./topologies',
                        help='Output directory for bulk generation')
    parser.add_argument('--workers', type=int, default=4,
                        help='Number of parallel workers for bulk generation')

    args = parser.parse_args()

    if args.bulk:
        # Bulk generation mode
        nodes = args.nodes
        replicas = args.replicas
        generate_bulk_topologies(args.output_dir, nodes, replicas, args.workers)
    else:
        # Single topology generation mode
        if not all([args.depth, args.services, args.replicas, args.nodes]):
            parser.error("For single topology generation, --depth, --services, --replicas, and --nodes are required")

        services_per_level = list(map(int, args.services.split(',')))
        nodes_list = args.nodes.split(',')

        yaml_output = generate_yaml(args.depth, services_per_level, args.replicas, nodes_list, args.scheduling)
        print(yaml_output)


if __name__ == "__main__":
    main()
