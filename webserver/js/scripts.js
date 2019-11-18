$(document).ready(function () {
    $("#sendForm").submit(function (event) {

        event.preventDefault();

        var message = document.getElementById("messageInput").value;

        if (message == "") {
            return
        }

        document.getElementById("messageInput").value = "";

        $.ajax({
            url: '/message',
            type: 'post',
            data: { text: message, destination: "" },
        });
    });
    $("#peerForm").submit(function (event) {

        event.preventDefault();

        var peer = document.getElementById("peerInput").value;
        document.getElementById("peerInput").value = "";

        if (peer == "") {
            return
        }

        $.ajax({
            url: '/node',
            type: 'post',
            data: peer,
        });
        updatePeersList()

    });

    $('#originList').dblclick(function (e) {
        var origin = e.target.textContent;
        var message = prompt("Enter the message to send to " + origin);
        if (message != null && message != "") {
            $.ajax({
                url: '/message',
                type: 'post',
                data: { text: message, destination: origin },
            });
        }
    });

    $('#searchList').dblclick(function (e) {
        var fileToDownload = e.target.textContent;
        var result = fileToDownload.split(" ");
        $.ajax({
            url: '/message',
            type: 'post',
            data: { file: result[0], request: result[1] },
        });

    });

    $('#buttonDownloadFile').click(function (e) {
        var requestHex = prompt("Enter the hexadecimal metahash of the file to download");
        if (requestHex != null && requestHex != "") {
            var dest = prompt("Enter the name of the peer who has the file");
            if (dest != null && dest != "") {
                var fileName = prompt("Enter the name of the file");
                if (fileName != null && fileName != "") {

                    $.ajax({
                        url: '/message',
                        type: 'post',
                        data: { text: "", destination: dest, file: fileName, request: requestHex },
                    });
                }
            }
        }
    });

    $('#buttonSearchFile').click(function (e) {
        var keywordsValue = prompt("Enter the keywords of the file you wanna search on the other nodes");
        if (keywordsValue != null && keywordsValue != "") {
            var budgetValue = prompt("Enter the initial budget for the search");
            if (budgetValue != null && budgetValue != "") {

                $.ajax({
                    url: '/message',
                    type: 'post',
                    data: { text: "", keywords: keywordsValue, budget: budgetValue },
                });
            }
        }
    });

    $("#fileInput").css('opacity', '0');

    $("#buttonFile").click(function (e) {
        e.preventDefault();
        $("#fileInput").trigger('click');
    });

    $("#fileInput").change(function () {
        var input = $(this).val().split(/(\\|\/)/g).pop()
        $.ajax({
            url: '/message',
            type: 'post',
            data: { text: "", destination: "", file: input },
        });
    });


    $.get("/id", function (data) {
        data = data.replace("\"", "").replace("\"", "")
        document.getElementById("peerID").innerHTML = "Peerster - ID: " + data;
    });

    window.setInterval(function () {
        function updateNodeBox() {
            $.get("/node", function (data) {
                var array = JSON.parse(data);

                var list = document.getElementById('peerList');
                while (list.hasChildNodes()) {
                    list.removeChild(list.firstChild)
                }

                for (el of array) {
                    var entry = document.createElement('li');
                    entry.appendChild(document.createTextNode(el));
                    list.appendChild(entry);
                }
            });
        }
        updateNodeBox()

        function updateOriginBox() {
            $.get("/origin", function (data) {
                var array = JSON.parse(data);

                var list = document.getElementById('originList');
                while (list.hasChildNodes()) {
                    list.removeChild(list.firstChild)
                }

                for (el of array) {
                    var entry = document.createElement('li');
                    entry.appendChild(document.createTextNode(el));
                    list.appendChild(entry);
                }
            });
        }
        updateOriginBox()

        function updateFileBox() {
            $.get("/file", function (data) {
                var jsonData = JSON.parse(data);
                var list = document.getElementById('fileList');

                for (el of jsonData) {
                    var entry = document.createElement('li');
                    var text = el["Name"] + ", " + el["Size"] + " KB " + el["MetaHash"]
                    entry.style.margin = "10px"
                    entry.appendChild(document.createTextNode(text));
                    list.appendChild(entry);
                }
            });
        }
        updateFileBox()

        function updateDownloadBox() {
            $.get("/download", function (data) {
                var jsonData = JSON.parse(data);
                var list = document.getElementById('downloadList');

                for (el of jsonData) {
                    var entry = document.createElement('li');
                    var text = el["Name"] + ", " + el["Size"] + " KB " + el["MetaHash"]
                    entry.style.margin = "10px"
                    entry.appendChild(document.createTextNode(text));
                    list.appendChild(entry);
                }
            });
        }
        updateDownloadBox()

        function updateSearchBox() {
            $.get("/search", function (data) {
                var jsonData = JSON.parse(data);
                var list = document.getElementById('searchList');

                while (list.hasChildNodes()) {
                    list.removeChild(list.firstChild)
                }

                for (el of jsonData) {
                    var entry = document.createElement('li');
                    var text = el["Name"] + " " + el["MetaHash"]
                    entry.style.margin = "10px"
                    entry.appendChild(document.createTextNode(text));
                    list.appendChild(entry);
                }
            });
        }
        updateSearchBox()

        $.get("/message", function (data) {
            var jsonData = JSON.parse(data);
            var list = document.getElementById('messageList');

            for (el of jsonData) {
                var entry = document.createElement('li');
                var text = "[" + el["Origin"] + "] " + el["Text"]
                entry.appendChild(document.createTextNode(text));
                list.appendChild(entry);
            }
        });
    }, 1000);
});