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

        // Create "contracts over time" chart.
        let cot = document.getElementById("contracts-over-time").getContext('2d');
        let contractsOverTime = new Chart(cot, {
            type: 'bar',
            data: {
                labels: ["Red", "Blue", "Yellow", "Green", "Purple", "Orange"],
                datasets: [{
                    label: '# of Votes',
                    data: [12, 19, 3, 5, 2, 3],
                    backgroundColor: [
                        'rgba(255, 99, 132, 0.2)',
                        'rgba(54, 162, 235, 0.2)',
                        'rgba(255, 206, 86, 0.2)',
                        'rgba(75, 192, 192, 0.2)',
                        'rgba(153, 102, 255, 0.2)',
                        'rgba(255, 159, 64, 0.2)'
                    ],
                    borderColor: [
                        'rgba(255,99,132,1)',
                        'rgba(54, 162, 235, 1)',
                        'rgba(255, 206, 86, 1)',
                        'rgba(75, 192, 192, 1)',
                        'rgba(153, 102, 255, 1)',
                        'rgba(255, 159, 64, 1)'
                    ],
                    borderWidth: 1
                }]
            },
            options: {
                maintainAspectRatio: false,
                scales: {
                    yAxes: [{
                        ticks: {
                            beginAtZero:true
                        }
                    }]
                }
            }
        });
    }
}

// Get data from metaserver.
xhttp.open("GET", "dashboard.json", true)
xhttp.send()
