var response = null;

var xhttp = new XMLHttpRequest();
xhttp.onreadystatechange = function() {
    if (this.readyState == 4 && this.status == 200) {
        response = JSON.parse(this.responseText);
        console.log(response);

        // Create nodes for each of the renters and providers.
        let nodes = [];
        for (let renter of response.renters) {
            nodes.push({id: renter.id, group: 0})
        }
        for (let provider of response.providers) {
            nodes.push({id: provider.id, group: 1})
        }

        // Create edges for each of the contracts between the renters and providers.
        let edges = [];
        for (let contract of response.contracts) {
            edges.push({from: contract.renterId, to: contract.providerId})
        }

        let nodeDataSet = new vis.DataSet(nodes);
        let edgeDataSet = new vis.DataSet(edges);

        let container = document.getElementById('mynetwork');
        let data = {
            nodes: nodeDataSet,
            edges: edgeDataSet,
        }
        let options = {};
        let network = new vis.Network(container, data, options);
    }
}

// Get data from metaserver.
xhttp.open("GET", "dashboard.json", true)
xhttp.send()
