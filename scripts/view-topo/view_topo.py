"""
Small utility script used to visualize service topologies, easier to read than the text file sometimes.
"""
import yaml
from graphviz import Digraph
import sys

with open(sys.argv[1], 'r') as file:
    data = yaml.safe_load(file)

dot = Digraph(comment='Service Topology')
dot.attr(rankdir='LR')

nodes = set(service['node'] for service in data['services'].values())
for node in nodes:
    with dot.subgraph(name=f'cluster_{node}') as c:
        c.attr(label=node, style='filled', color='lightgrey')

        for service_name, service_info in data['services'].items():
            if service_info['node'] == node:
                # Create a cluster for each service
                with c.subgraph(name=f'cluster_{service_name}') as service_cluster:
                    service_cluster.attr(label=service_name, style='rounded,filled', color='lightblue')
                    # Add replicas as nodes within the service cluster
                    for i in range(service_info['replicas']):
                        replica_name = f"{service_name}_replica_{i+1}"
                        replica_label = f"Replica {i+1}\n{service_info['processing_delay']}"
                        service_cluster.node(replica_name, replica_label, shape='ellipse')

                    service_cluster.node(service_name, style='invis', shape='point')

for service_name, service_info in data['services'].items():
    for out_service in service_info.get('out_services', []):
        dot.edge(service_name, out_service)

dot.render('topo', format='png', cleanup=True)
print("Saved as topo.png")
