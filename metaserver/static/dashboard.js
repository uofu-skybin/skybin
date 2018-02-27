var response = null;

let chartColors = {
	red: 'rgb(255, 99, 132)',
	orange: 'rgb(255, 159, 64)',
	yellow: 'rgb(255, 205, 86)',
	green: 'rgb(75, 192, 192)',
	blue: 'rgb(54, 162, 235)',
	purple: 'rgb(153, 102, 255)',
	grey: 'rgb(231,233,237)'
};

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

        const weekdays = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
        let currentWeekday = new Date().getDay();
        
        let labels = [];
        for (let i = 0; i < 7; i++) {
            labels[i] = weekdays[((currentWeekday + 6) - i) % 7];
        }

        // Get number of contracts formed over last 7 weekdays.
        let numContracts = [];
        for (let i = 0; i < 7; i++) {
            let currDate = new Date();
            currDate.setDate(currDate.getDate() - 7 + i);

            let currNumber = 0;
            for (let contract of response.contracts) {
                let contractDate = new Date(contract.startDate);
                if (contractDate.getDate() == currDate.getDate() && contractDate.getFullYear() == currDate.getFullYear() && 
                  contractDate.getMonth() == currDate.getMonth()) {
                    currNumber++;
                }
            }
            
            numContracts[i] = currNumber;
        }

        // Create "contracts over time" chart.
        let cot = document.getElementById("contracts-over-time").getContext('2d');
        let contractsOverTime = new Chart(cot, {
            type: 'line',
            data: {
                // labels: ["Red", "Blue", "Yellow", "Green", "Purple", "Orange"],
                labels: labels.reverse(),
                datasets: [{
                    label: '# of Contracts',
                    // data: [12, 19, 3, 5, 2, 3],
                    data: numContracts,
                    backgroundColor: chartColors.blue,
                    borderColor: chartColors.blue,
                    fill: false,
                }]
            },
            options: {
                maintainAspectRatio: false,
                scales: {
                    yAxes: [{
                        ticks: {
                            beginAtZero:true,
                            stepSize:1,
                        },
                    }]
                },
                legend: {
                    display: false
                },
                title: {
                    display: true,
                    text: "Number of Contracts Established in Last 7 Days"
                }
            }
        });

        let uot = document.getElementById("uploads-over-time").getContext('2d');
        let uploadsOverTime = new Chart(uot, {
            type: 'line',
            data: {
                // labels: ["Red", "Blue", "Yellow", "Green", "Purple", "Orange"],
                labels: labels.reverse(),
                datasets: [{
                    label: '# of Contracts',
                    // data: [12, 19, 3, 5, 2, 3],
                    data: numContracts,
                    backgroundColor: chartColors.blue,
                    borderColor: chartColors.blue,
                    fill: false,
                }]
            },
            options: {
                maintainAspectRatio: false,
                scales: {
                    yAxes: [{
                        ticks: {
                            beginAtZero:true,
                            stepSize:1,
                        },
                    }]
                },
                legend: {
                    display: false
                },
                title: {
                    display: true,
                    text: "Uploads Over Time"
                }
            }
        });

        let fsd = document.getElementById("file-size-distribution").getContext('2d');
        let fileSizeDistribution = new Chart(fsd, {
            type: 'line',
            data: {
                // labels: ["Red", "Blue", "Yellow", "Green", "Purple", "Orange"],
                labels: labels.reverse(),
                datasets: [{
                    label: '# of Contracts',
                    // data: [12, 19, 3, 5, 2, 3],
                    data: numContracts,
                    backgroundColor: chartColors.blue,
                    borderColor: chartColors.blue,
                    fill: false,
                }]
            },
            options: {
                maintainAspectRatio: false,
                scales: {
                    yAxes: [{
                        ticks: {
                            beginAtZero:true,
                            stepSize:1,
                        },
                    }]
                },
                legend: {
                    display: false
                },
                title: {
                    display: true,
                    text: "File Size Distribution"
                }
            }
        });
    }
}

// Get data from metaserver.
xhttp.open("GET", "dashboard.json", true)
xhttp.send()
