// Colors to use when creating charts.
const chartColors = {
	red: 'rgb(255, 99, 132)',
	orange: 'rgb(255, 159, 64)',
	yellow: 'rgb(255, 205, 86)',
	green: 'rgb(75, 192, 192)',
	blue: 'rgb(54, 162, 235)',
	purple: 'rgb(153, 102, 255)',
    grey: 'rgb(231,233,237)',
    darkGrey: 'rgb(128,128,128)',
    brightGreen: '#66FF00'
};

// Current set of data from the metaserver.
let response = null;

// Renter and provider mappings used to display their details
let renters = {};
let providers = {};

$(document).ready(function() {
    setupPage();
    setInterval(updatePage, 30000);
    setInterval(updateRefreshTime, 5000);

    $('#node-id').click(copyToClipboard);
});

function copyToClipboard(event) {
    $('#copy-alert').show(0).delay(1000).hide(0);
    var $temp = $("<input>");
    $("body").append($temp);
    $temp.val($(this).text()).select();
    document.execCommand("copy");
    $temp.remove();
}

function setupPage() {
    var xhttp = new XMLHttpRequest();
    xhttp.onreadystatechange = function() {
        if (this.readyState == 4 && this.status == 200) {
            response = JSON.parse(this.responseText);

            setupNetworkAndNodeDetails();
            createContractsOverTime();
            createUploadsOverTime();
            createFileSizeDistribution();
        }
    }
    // Get data from metaserver.
    xhttp.open("GET", "dashboard.json", true)
    xhttp.send()

    setupLegend();
}

function setupLegend() {
    let container = document.getElementById('network-legend');
    let x = 0;
    let y = 0;
    let nodes = [
        {id: 1, x: x, y: y, group: 0, label: "Renter", fixed: true, physics: false},
        {id: 2, x: x, y: y + 90, group: 1, label: "Provider", fixed: true, physics: false},
    ];
    let dataSet = {
        nodes: new vis.DataSet(nodes),
        edges: new vis.DataSet([])
    };
    let options = {
        nodes: {
            shape: 'dot',
        },
        interaction: {
            dragView: false,
            selectable: false
        },
        groups: {
            0: {
                color: chartColors.blue
            },
            1: {
                color: chartColors.orange
            }
        }
    };
    network = new vis.Network(container, dataSet, options);
    network.fit({nodes: [1,2]});
    let pos = network.getViewPosition();
    let scale = network.getScale();
    network.moveTo({position: {x: x, y: pos.y + 5}, scale: scale * 0.9});
}

let refreshTime = new Date();

function updateRefreshTime() {
    let secondsPassed = Math.floor((new Date() - refreshTime) / 1000);
    $('#refresh-time').text(secondsPassed);
}

function updatePage() {
    console.log('Updating!');

    $('#refresh-button').addClass('fa-spin');

    
    var xhttp = new XMLHttpRequest();
    xhttp.onreadystatechange = function() {
        if (this.readyState == 4 && this.status == 200) {
            response = JSON.parse(this.responseText);

            updateNetworkAndNodeDetails();
            updateContractsOverTime(7);
            updateUploadsOverTime(7);
            updateFileSizeDistribution();
            updateNodeInfo();
            $('#refresh-button').removeClass('fa-spin');
            refreshTime = new Date();
            updateRefreshTime();
        }
    }
    // Get data from metaserver.
    xhttp.open("GET", "dashboard.json", true)
    xhttp.send()
}

// Create nodes for each of the renters and providers.
let network = null;

let nodeDataSet = null;
let edgeDataSet = null;

function setupNetworkAndNodeDetails() {
    /**
     * Build the network graph and node details pane.
     */
    let nodes = [];

    renters = {};
    for (let renter of response.renters) {
        nodes.push({id: renter.id, group: 0})
        renters[renter.id] = renter;
    }

    providers = {};
    for (let provider of response.providers) {
        nodes.push({id: provider.id, group: 1})
        providers[provider.id] = provider;
    }

    // Create edges for each of the contracts between the renters and providers.
    let edges = [];
    let edgeSet = {};
    for (let contract of response.contracts) {
        let edgeId = contract.renterId + ' ' + contract.providerId;
        if (!edgeSet[edgeId]) {
            edges.push({
                id: edgeId,
                from: contract.renterId, 
                to: contract.providerId,
                chosen: {
                    edge: function(values, id, selected, hovering) {
                        values.dashes = true;
                        values.width *= 3;
                    }
                }
            })
            edgeSet[edgeId] = true;
        }
    }

    // Build our network.
    nodeDataSet = new vis.DataSet(nodes);
    edgeDataSet = new vis.DataSet(edges);
    let container = document.getElementById('my-network');
    let dataSet = {
        nodes: nodeDataSet,
        edges: edgeDataSet,
    }
    let options = {
        interaction: {
            selectConnectedEdges: false
        },
        nodes: {
            chosen: {
                node: function(values, id, selected, hovering) {
                    values.color = chartColors.brightGreen;
                }
            }
        },
        edges: {
            chosen: {
                edge: function(values, id, selected, hovering) {
                    values.dashes = true;
                }
            }
        },
        groups: {
            0: {
                color: chartColors.blue
            },
            1: {
                color: chartColors.orange
            }
        },
        edges: {
            color: {
                color: chartColors.darkGrey
            }
        },
    };
    network = new vis.Network(container, dataSet, options);

    // Set up events so we retrieve information for clicked node.
    network.on("click", function(params) {
        let nodeId = network.getNodeAt({x: params.pointer.DOM.x, y: params.pointer.DOM.y});
        if (!nodeId) {
            return
        } 
        showNodeInfo(nodeId);
    });
}

function updateNetworkAndNodeDetails() {
    /**
     * Update the network graph.
     */
    let nodes = [];

    renters = {};
    for (let renter of response.renters) {
        nodes.push({id: renter.id, group: 0})
        renters[renter.id] = renter;
    }

    providers = {};
    for (let provider of response.providers) {
        nodes.push({id: provider.id, group: 1})
        providers[provider.id] = provider;
    }

    // If any of the nodes are not already present, add them to the network.
    for (let node of nodes) {
        let existingNode = nodeDataSet.get(node.id)
        if (!existingNode) {
            nodeDataSet.add(node);
        }
    }

    // If any of the network's nodes are not present in renters or providers, remove them.
    for (let id of nodeDataSet.getIds()) {
        if (!renters[id] && !providers[id]) {
            nodeDataSet.remove(id);
        }
    }

    // Create edges for each of the contracts between the renters and providers.
    let edges = [];
    let edgeSet = {};
    for (let contract of response.contracts) {
        let edgeId = contract.renterId + ' ' + contract.providerId;
        if (!edgeSet[edgeId]) {
            edgeSet[edgeId] = true;
            edges.push({
                id: edgeId,
                from: contract.renterId, 
                to: contract.providerId
            })
        }
    }

    // If any of the edges are not present in the graph, add them.
    for (let edge of edges) {
        let existingEdge = edgeDataSet.get(edge.id)
        if (!existingEdge) {
            edgeDataSet.add(edge);
        }
    }

    // If any of the network's edges are not present in renters or providers, remove them.
    for (let id of edgeDataSet.getIds()) {
        if (!edgeSet[id]) {
            edgeDataSet.remove(id);
        }
    }
}

// Set of blocks that we are currently auditing.
let auditing = {};

function showNodeInfo(nodeId) {
    /** 
     * Display information about the node in the given parameters.
    */
    let renter = renters[nodeId];
    if (renter != undefined) {
        $('#node-id').text(renter.id);
        $('#node-type').text('renter');
        $('#renter-name').text(renter.alias);
        $('#node-balance').text(renter.balance / 1000);

        let numberOfFiles = 0;
        let storageUsed = 0;
        $('#file-list').empty()
        for (let file of response.files) {
            if (file.ownerId == renter.id) {
                numberOfFiles++;
                for (let version of file.versions) {
                    storageUsed += version.uploadSize;
                }

                let li = $('<li>')

                let isCorrupt = false;
                if (file.versions.length > 0) {
                    for (let block of file.versions[file.versions.length - 1].blocks) {
                        if (!block.auditPassed) {
                            isCorrupt = true;
                            break;
                        }
                    }
                }
                if (isCorrupt) {
                    li.append($('<i>', {
                        'class': 'indicator fas fa-times-circle text-danger',
                        'title': 'contains corrupt blocks'
                    }));
                } else {
                    li.append($('<i>', {
                        'class': 'indicator fas fa-check-circle text-success',
                        'title': 'all blocks verified'
                    }));
                }

                let $verifyButton = $('<i>', {
                    'class': 'fas fa-question-circle text-primary can-click',
                    'title': 'verify file integrity'
                })
                $verifyButton.on(
                    'click',
                    { file: file},
                    checkFileIntegrity
                );
                li.append(' ');
                li.append($verifyButton);

                let $name = $('<span>', {'class': 'clickable-item'});
                $name.append(' ' + file.name);
                $name.click(() => {
                    showFileContractsAndLocations(renter.id, file.id);
                });

                li.append($name);

                $('#file-list').append(li);
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

    let provider = providers[nodeId];
    if (provider != undefined) {
        $('#node-id').text(provider.id);
        $('#node-type').text('provider');
        $('#storage-available').text(bytesToSize(provider.spaceAvail));
        $('#node-balance').text(provider.balance / 1000);

        let amountReserved = 0;
        for (let contract of response.contracts) {
            if (contract.providerId == provider.id) {
                amountReserved += contract.storageSpace;
            }
        }
        $('#storage-leased').text(bytesToSize(amountReserved));
        $('#storage-offering').text(bytesToSize(amountReserved + provider.spaceAvail));

        $('#storage-rate').text(provider.storageRate / 1000);

        // Check if the user has expanded any of the existing list items.
        let expandedSet = {};
        for (let item of $('#file-list').children()) {
            let name = '';
            let isExpanded = false;
            for (let span of $(item).children()) {
                if ($(span).hasClass('block-list')) {
                    if ($(span).css('display') != 'none') {
                        isExpanded = true;
                    }
                } else {
                    name = $(span).text();
                }
            }
            if (isExpanded) {
                expandedSet[name] = true;
            }
        }
        $('#file-list').empty()
        for (let file of response.files) {
            if (file.versions.length == 0) {
                continue;
            }

            let latestVersion = file.versions[file.versions.length - 1];
            let listItem = $('<li>');
            if (!expandedSet[file.name]) {
                listItem.addClass('clickable-item');
            }
            
            let bulletSpan = $('<span>');
            bulletSpan.attr('class', 'bullet-span');
            if (!expandedSet[file.name]) {
                bulletSpan.append('<i class="fas fa-angle-right"></i> ');
            } else {
                bulletSpan.append('<i class="fas fa-angle-down"></i> ');
            }
            listItem.append(bulletSpan);

            let nameSpan = $('<span>');
            nameSpan.append(file.name);
            nameSpan.click(showOrHideBlocks);


            listItem.append(nameSpan);

            let span = $('<span>', {"class": "block-list text-muted"})
            if (!expandedSet[file.name]) {
                span.hide();
            }
            span.append('<br>Block IDs:<br>')

            let blockList = $('<ul>', {"class": "block-id-list"});
            let blockStored = false;
            for (let block of latestVersion.blocks) {
                if (block.location.providerId == provider.id) {
                    blockStored = true;
                    let listItem = $('<li>');
                    if (block.auditPassed) {
                        listItem.append('<i title="block verified" class="indicator fas fa-check-circle text-success"></i> ');
                    } else {
                        listItem.append('<i title="block corrupt" class="indicator fas fa-times-circle text-danger"></i> ');
                    }

                    let integrityIcon = $('<i>', {'class': 'can-click fas'})
                    integrityIcon.on(
                        'click',
                        { fileId: file.id, blockId: block.id},
                        checkIntegrity
                    );
                    integrityIcon.addClass('text-primary');
                    if (!auditing[block.id]) {
                        integrityIcon.addClass('fa-question-circle');
                        integrityIcon.prop('title', 'check block integrity')
                    } else {
                        integrityIcon.addClass('fa-spinner');
                        integrityIcon.addClass('fa-spin');
                        integrityIcon.prop('title', 'checking block integrity')
                    }

                    listItem.append(integrityIcon);

                    let idSpan = $('<span>', {'class': 'can-copy'});
                    idSpan.append(block.id);
                    idSpan.click(copyToClipboard);
                    listItem.append(idSpan);
                    blockList.append(listItem);
                }
            }

            span.append(blockList);
            listItem.append(span);

            if (blockStored) {
                $('#file-list').append(listItem);
            }
        }

        $('#renter-info').hide();
        $('#provider-info').show();
        $("#file-list-container").css("max-height", $("#node-info").height()-$("#provider-info").height()-$("#general-info").height());
    }
}

function checkFileIntegrity(event) {
    let $this = $(this);
    let file = event.data.file;

    if (file.versions.length == 0) {
        return;
    }
    let latestVersion = file.versions[file.versions.length - 1];

    let numBlocks = latestVersion.blocks.length;
    $this.removeClass('fa-question-circle');
    $this.addClass('fa-spinner');
    $this.addClass('fa-spin');
    $this.prop('title', 'checking file integrity')

    let numChecked = 0;
    let isCorrupt = false;

    for (let block of latestVersion.blocks) {
        var xhttp = new XMLHttpRequest();
        xhttp.onreadystatechange = function() {
            if (this.readyState == 4 && this.status == 200) {
                let response = JSON.parse(this.responseText);
                if (!response.success) {
                    isCorrupt = true;
                }
                numChecked++;

                if (numChecked == numBlocks) {
                    $this.removeClass('fa-spinner');
                    $this.removeClass('fa-spin');
                    $this.addClass('fa-question-circle');

                    console.log(isCorrupt);
                    if (!isCorrupt) {
                        $this.siblings('.indicator').removeClass('fa-times-circle');
                        $this.siblings('.indicator').removeClass('text-danger');
                        $this.siblings('.indicator').addClass('fa-check-circle');
                        $this.siblings('.indicator').addClass('text-success');
                    } else {
                        $this.siblings('.indicator').removeClass('fa-check-circle');
                        $this.siblings('.indicator').removeClass('text-success');
                        $this.siblings('.indicator').addClass('fa-times-circle');
                        $this.siblings('.indicator').addClass('text-danger');
                    }
                }
            }
        }
    // Get data from metaserver.
        xhttp.open("POST", "/dashboard/audit/" + file.id + "/" + block.id, true)
        xhttp.send()
    }
}

function checkIntegrity(event) {
    let $this = $(this);
    $this.removeClass('fa-question-circle');
    $this.addClass('fa-spinner');
    $this.addClass('fa-spin');
    $this.prop('title', 'checking block integrity')
    auditing[event.data.blockId] = true;

    var xhttp = new XMLHttpRequest();
    xhttp.onreadystatechange = function() {
        if (this.readyState == 4 && this.status == 200) {
            delete auditing[event.data.blockId];

            $this.removeClass('fa-spinner');
            $this.removeClass('fa-spin');
            $this.addClass('fa-question-circle');

            let response = JSON.parse(this.responseText);
            if (response.success) {
                $this.siblings('.indicator').removeClass('fa-times-circle');
                $this.siblings('.indicator').removeClass('text-danger');
                $this.siblings('.indicator').addClass('fa-check-circle');
                $this.siblings('.indicator').addClass('text-success');
            } else {
                $this.siblings('.indicator').removeClass('fa-check-circle');
                $this.siblings('.indicator').removeClass('text-success');
                $this.siblings('.indicator').addClass('fa-times-circle');
                $this.siblings('.indicator').addClass('text-danger');
            }
        }
    }
    // Get data from metaserver.
    xhttp.open("POST", "/dashboard/audit/" + event.data.fileId + "/" + event.data.blockId, true)
    xhttp.send()
}

function updateNodeInfo() {
    showNodeInfo($('#node-id').text());
}

function showOrHideBlocks(event) {
    if ($(this).siblings('.block-list').is(':visible')) {
        $(this).siblings('.bullet-span').html('<i class="fas fa-angle-right"></i> ');
        $(this).parent().addClass('clickable-item');
    } else {
        $(this).siblings('.bullet-span').html('<i class="fas fa-angle-down"></i> ');
        $(this).parent().removeClass('clickable-item');
    }
    $(this).siblings('.block-list').slideToggle(100);
}

function showFileContractsAndLocations(renterId, fileId) {
    network.setSelection({nodes: [], edges: []});
    setTimeout(() => {
        let nodesToSelect = [renterId];
        let edgesToSelect = [];
        for (let file of response.files) {
            if (file.id == fileId) {
                if (file.versions.length > 0) {
                    let latestVersion = file.versions[file.versions.length - 1];
                    for (let block of latestVersion.blocks) {
                        nodesToSelect.push(block.location.providerId);
                        edgesToSelect.push(renterId + ' ' + block.location.providerId);
                    }
                }
                break;
            }
        }

        network.setSelection({
            nodes: nodesToSelect,
            edges: edgesToSelect,
        },
        {
            unselectAll: true,
            highlightEdges: false
        });
    }, 100);
}

function getPreviousTimes() {
    let times = [];
    let numTicks = 7;
    let currTime = new Date();
    if (currTime.getMinutes() < 30) {
        currTime.setMinutes(30);
    } else {
        currTime.setHours(currTime.getHours() + 1);
        currTime.setMinutes(0);
    }
    currTime.setSeconds(0);
    for (let i = 0; i < numTicks; i++) {
        let nextTime = new Date(currTime - 30 * 60 * 1000 * i);
        times.push(nextTime);
    }
    return times.reverse();
}

let contractsOverTime = null;

function createContractsOverTime() {
    /** 
     * Create the contracts per day chart
    */
    let labelsAndData = calculateContractsOverTime();
    let labels = labelsAndData[0];
    let data = labelsAndData[1];

    // Create "contracts over time" chart.
    let cot = document.getElementById("contracts-over-time").getContext('2d');
    contractsOverTime = new Chart(cot, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: '# of Reservations',
                data: data,
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
                    type: 'time',
                }],
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
                text: "Storage Reservations in Last 3 Hours"
            }
        }
    });
}

function updateContractsOverTime() {
    let labelsAndData = calculateContractsOverTime();
    let labels = labelsAndData[0];
    let data = labelsAndData[1];

    contractsOverTime.labels = labels;
    contractsOverTime.data.datasets[0].data = data;

    contractsOverTime.update();
}

function calculateContractsOverTime() {
    let times = getPreviousTimes();
    let contractNumbers = new Array(times.length).fill(0);

    for (let contract of response.contracts) {
        let contractDate = new Date(contract.startDate);
        for (let i = 0; i < times.length - 1; i++) {
            if (contractDate > times[i] && contractDate <= times[i + 1]) {
                contractNumbers[i]++;
            }
        }
    }

    times.splice(times.length - 1, 1);
    contractNumbers.splice(contractNumbers.length - 1, 1);
    let returnVal = [times, contractNumbers];
    return returnVal;
}

let uploadsOverTime = null;

function createUploadsOverTime() {
    let labelsAndData = calculateUploadsOverTime();
    let labels = labelsAndData[0];
    let data = labelsAndData[1];

    let uot = document.getElementById("uploads-over-time").getContext('2d');
    uploadsOverTime = new Chart(uot, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: '# of Uploads',
                data: data,
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
                    },
                }]
            },
            legend: {
                display: false
            },
            title: {
                display: true,
                text: "Uploads in Last 3 Hours"
            }
        }
    });
}

function updateUploadsOverTime() {
    let labelsAndData = calculateUploadsOverTime();
    let labels = labelsAndData[0];
    let data = labelsAndData[1];

    uploadsOverTime.labels = labels;
    uploadsOverTime.data.datasets[0].data = data;

    uploadsOverTime.update();
}

function calculateUploadsOverTime() {
    let times = getPreviousTimes();
    let uploadCounts = new Array(times.length).fill(0);

    for (let file of response.files) {
        for (let version of file.versions) {
            let versionDate = new Date(version.uploadTime);
            for (let i = 0; i < times.length - 1; i++) {
                if (versionDate > times[i] && versionDate <= times[i+1]) {
                    uploadCounts[i]++;
                }
            }
        }
    }

    times.splice(times.length - 1, 1);
    uploadCounts.splice(uploadCounts.length - 1, 1);
    let returnVal = [times, uploadCounts];
    return returnVal;
}

let fileSizeDistribution = null;

function createFileSizeDistribution() {
    let labelsAndData = calculateFileSizeDistribution();
    let labels = labelsAndData[0];
    let data = labelsAndData[1];

    let fsd = document.getElementById("file-size-distribution").getContext('2d');
    fileSizeDistribution = new Chart(fsd, {
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

function updateFileSizeDistribution() {
    let labelsAndData = calculateFileSizeDistribution();
    let data = labelsAndData[1];

    fileSizeDistribution.data.datasets[0].data = data;

    fileSizeDistribution.update();
}

function calculateFileSizeDistribution() {
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
    for (let file of response.files) {
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

    return [labels, data];
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
