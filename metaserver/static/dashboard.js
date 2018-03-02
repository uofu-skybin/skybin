$(document).ready(function() {
    
  });

let response = null;

// Colors to use when creating charts.
let chartColors = {
	red: 'rgb(255, 99, 132)',
	orange: 'rgb(255, 159, 64)',
	yellow: 'rgb(255, 205, 86)',
	green: 'rgb(75, 192, 192)',
	blue: 'rgb(54, 162, 235)',
	purple: 'rgb(153, 102, 255)',
	grey: 'rgb(231,233,237)'
};

// Renter and provider mappings used to display their details
let renters = {};
let providers = {};

function showNodeInfo(params) {
    /** 
     * Display information about the node in the given parameters.
    */
    let renter = renters[params.nodes[0]];
    if (renter != undefined) {
        $('#node-id').text(renter.id);
        $('#node-type').text('renter');
        $('#renter-name').text(renter.alias);

        let numberOfFiles = 0;
        let storageUsed = 0;
        $('#file-list').empty()
        for (let file of response.files) {
            if (file.ownerId == renter.id) {
                numberOfFiles++;
                for (let version of file.versions) {
                    storageUsed += version.uploadSize;
                }
                $('#file-list').append('<li>' + file.name + '</li>');
            }
        }

        let storageReserved = 0;
        for (let contract of response.contracts) {
            if (contract.renterId == renter.id) {
                storageReserved += contract.storageSpace;
            }
        }
        $('#files-uploaded').text(numberOfFiles);
        $('#storage-used').text(bytesToSize(storageUsed));
        $('#storage-reserved').text(bytesToSize(storageReserved));
        $('#storage-available-renter').text(bytesToSize(storageReserved - storageUsed));

        $('#provider-info').hide();
        $('#renter-info').show();
        $("#file-list-container").css("max-height", $("#node-info").height()-$("#renter-info").height()-$("#general-info").height());
    }

    let provider = providers[params.nodes[0]];
    if (provider != undefined) {
        $('#node-id').text(provider.id);
        $('#node-type').text('provider');
        $('#storage-available').text(bytesToSize(provider.spaceAvail));

        let amountReserved = 0;
        for (let contract of response.contracts) {
            if (contract.providerId == provider.id) {
                amountReserved += contract.storageSpace;
            }
        }
        $('#storage-leased').text(bytesToSize(amountReserved));
        $('#storage-offering').text(bytesToSize(amountReserved + provider.spaceAvail));

        $('#file-list').empty()
        for (let file of response.files) {
            for (let version of file.versions) {
                let fileStored = false;
                for (let block of version.blocks) {
                    if (block.location.providerId == provider.id) {
                        $('#file-list').append('<li>' + file.name + '</li>');
                        fileStored = true;
                        break;
                    }
                }
                if (fileStored) {
                    break;
                }
            }
        }

        $('#renter-info').hide();
        $('#provider-info').show();
        $("#file-list-container").css("max-height", $("#node-info").height()-$("#provider-info").height()-$("#general-info").height());
    }
}

function setupNetworkAndNodeDetails(data) {
    /**
     * Build the network graph and node details pane.
     */

    // Create nodes for each of the renters and providers.
    let nodes = [];

    renters = {};
    for (let renter of data.renters) {
        nodes.push({id: renter.id, group: 0})
        renters[renter.id] = renter;
    }

    providers = {};
    for (let provider of data.providers) {
        nodes.push({id: provider.id, group: 1})
        providers[provider.id] = provider;
    }

    // Create edges for each of the contracts between the renters and providers.
    let edges = [];
    for (let contract of data.contracts) {
        edges.push({from: contract.renterId, to: contract.providerId})
    }

    // Build our network.
    let nodeDataSet = new vis.DataSet(nodes);
    let edgeDataSet = new vis.DataSet(edges);
    let container = document.getElementById('my-network');
    let dataSet = {
        nodes: nodeDataSet,
        edges: edgeDataSet,
    }
    let options = {};
    let network = new vis.Network(container, dataSet, options);

    // Set up events so we retrieve information for clicked node.
    network.on("selectNode", showNodeInfo);
}

function getPreviousDays(numDays) {
    /** 
     * Create an array containing the specified number of days, moving backward starting with the current day.
    */
    let days = [];
    for (let i = 0; i < numDays; i++) {
        let currDate = new Date();
        currDate.setDate(currDate.getDate() - (numDays - 1) + i);
        days.push(currDate);
    }
    return days;
}

function createContractsOverTime(contracts, numberOfDays) {
    /** 
     * Create the contracts per day chart
    */
    let days = getPreviousDays(numberOfDays);
    let dates = {};
    for (let day of days) {
        dates[day.toDateString()] = 0;
    }

    for (let contract of contracts) {
        let contractDate = new Date(contract.startDate).toDateString();
        if (dates[contractDate] != undefined) {
            dates[contractDate]++;
        }
    }

    let numberOfContractsPerDay = [];
    for (let i = 0; i < days.length; i++) {
        numberOfContractsPerDay[i] = dates[days[i].toDateString()];
    }

    // Create "contracts over time" chart.
    let cot = document.getElementById("contracts-over-time").getContext('2d');
    let contractsOverTime = new Chart(cot, {
        type: 'line',
        data: {
            labels: days,
            datasets: [{
                label: '# of Reservations',
                data: numberOfContractsPerDay,
                backgroundColor: chartColors.blue,
                borderColor: chartColors.blue,
                fill: false,
                lineTension: 0
            }]
        },
        options: {
            maintainAspectRatio: false,
            scales: {
                xAxes: [{
                    type: 'time'
                }],
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
                text: "Storage Reservations Per Day"
            }
        }
    });
}

function createUploadsOverTime(files, numberOfDays) {
    let days = getPreviousDays(numberOfDays);
    let dates = {};
    for (let day of days) {
        dates[day.toDateString()] = 0;
    }

    for (let file of files) {
        for (let version of file.versions) {
            let versionDate = new Date(version.uploadTime).toDateString();
            if (dates[versionDate] != undefined) {
                dates[versionDate]++;
            }
        }
    }

    let uploadsPerDay = [];
    for (let i = 0; i < days.length; i++) {
        uploadsPerDay[i] = dates[days[i].toDateString()];
    }

    let uot = document.getElementById("uploads-over-time").getContext('2d');
        let uploadsOverTime = new Chart(uot, {
            type: 'line',
            data: {
                labels: days,
                datasets: [{
                    label: '# of Uploads',
                    data: uploadsPerDay,
                    backgroundColor: chartColors.blue,
                    borderColor: chartColors.blue,
                    fill: false,
                    lineTension:0,
                }]
            },
            options: {
                maintainAspectRatio: false,
                scales: {
                    xAxes: [{
                        type: 'time'
                    }],
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
                    text: "Uploads Per Day"
                }
            }
        });
}

function bytesToSize(value) {
    if (value === undefined || value === null) {
        return '';
    }

    let amt = value;
    let suffix = 'B';

    if (value >= 1e12) {
        amt = value / 1e12;
        suffix = 'TB';
    } else if (value >= 1e9) {
        amt = value / 1e9;
        suffix = 'GB';
    } else if (value >= 1e6) {
        amt = value / 1e6;
        suffix = 'MB';
    } else if (value >= 1e3) {
        amt = value / 1e3;
        suffix = 'KB';
    }

    if (amt % 1 !== 0) {
        amt = parseFloat(amt.toFixed(1));
    }

    return amt + suffix;
}

function createFileSizeDistribution(files) {
    const startSize = 1000000; // 10 Mb
    const maxSize = 5000000000; // 5 Gb

    let sizesToNumber = {};
    let fileSizes = [];

    let currSize = startSize;
    while (currSize < maxSize) {
        sizesToNumber[currSize] = 0;
        fileSizes.push(currSize);
        currSize *= 2;
    }

    // Round each size to the nearest 10 MB, then place them in the file sizes dictionary.
    for (let file of files) {
        for (let version of file.versions) {
            for (let i = 0; i < fileSizes.length; i++) {
                let currSize = fileSizes[i];
                if (version.size < currSize) {
                    sizesToNumber[currSize]++;
                    break;
                }
            }
        }
    }

    // Clean up the labels and create adataset
    let labels = [];
    let data = [];
    for (let i = 0; i < fileSizes.length; i++) {
        labels[i] = bytesToSize(fileSizes[i]);
        data[i] = sizesToNumber[fileSizes[i]];
    }

    let fsd = document.getElementById("file-size-distribution").getContext('2d');
        let fileSizeDistribution = new Chart(fsd, {
            type: 'line',
            data: {
                labels: labels,
                datasets: [{
                    label: '# of Files',
                    data: data,
                    backgroundColor: chartColors.blue,
                    borderColor: chartColors.blue,
                    fill: false,
                    lineTension:0
                }]
            },
            options: {
                maintainAspectRatio: false,
                scales: {
                    yAxes: [{
                        ticks: {
                            beginAtZero:true,
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

var xhttp = new XMLHttpRequest();
xhttp.onreadystatechange = function() {
    // Dictionaries to hold renter and provider information (we use this to retrieve it quickly).
    let renters = {};
    let providers = {};

    if (this.readyState == 4 && this.status == 200) {
        response = JSON.parse(this.responseText);
        console.log(response);

        setupNetworkAndNodeDetails(response);

        createContractsOverTime(response.contracts, 7);

        createUploadsOverTime(response.files, 7);

        createFileSizeDistribution(response.files);
    }
}

// Get data from metaserver.
xhttp.open("GET", "dashboard.json", true)
xhttp.send()