var response = null;

var xhttp = new XMLHttpRequest();
xhttp.onreadystatechange = function() {
    // Dictionaries to hold renter and provider information (we use this to retrieve it quickly).
    let renters = {};
    let providers = {};

    if (this.readyState == 4 && this.status == 200) {
        response = JSON.parse(this.responseText);
        console.log(response);

        // Create nodes for each of the renters and providers.
        let nodes = [];

        let renters = {};
        for (let renter of response.renters) {
            nodes.push({id: renter.id, group: 0})
            renters[renter.id] = renter;
        }

        let providers = {};
        for (let provider of response.providers) {
            nodes.push({id: provider.id, group: 1})
            providers[provider.id] = provider;
        }

        // Create edges for each of the contracts between the renters and providers.
        let edges = [];
        for (let contract of response.contracts) {
            edges.push({from: contract.renterId, to: contract.providerId})
        }

        // Build our network.
        let nodeDataSet = new vis.DataSet(nodes);
        let edgeDataSet = new vis.DataSet(edges);
        let container = document.getElementById('my-network');
        let data = {
            nodes: nodeDataSet,
            edges: edgeDataSet,
        }
        let options = {};
        let network = new vis.Network(container, data, options);

        // Set up events so we retrieve information for clicked node.
        network.on("selectNode", function (params) {
            let renter = renters[params.nodes[0]];
            if (renter != undefined) {
                $('#node-id').text(renter.id);
                $('#node-type').text('renter');
            }

            let provider = providers[params.nodes[0]];
            if (provider != undefined) {
                $('#node-id').text(provider.id);
                $('#node-type').text('provider');
            }
        });
    }
}

// Get data from metaserver.
xhttp.open("GET", "dashboard.json", true)
xhttp.send()
