import 'package:flutter/material.dart';
import 'package:sse/client/sse_client.dart';
import 'dart:convert';
import 'package:intl/intl.dart';
import 'package:http/http.dart' as http;

final url = 'https://control-hub.home.rsmachiner.com/send';
final controlURL = 'https://control-hub.home.rsmachiner.com/config/home/smoker-pi/configs';
final eventURL = 'https://events.home.rsmachiner.com/stream/home/smoker-pi/readings';


class Data{
  final String id;
  final double c;
  final double f;
  Data({
    this.id,
    this.c,
    this.f
  });

  factory Data.fromJson(Map<String, dynamic> json) {
    return Data(
      id: json['id'],
      c: json['c'].toDouble(),
      f: json['f'].toDouble()
    );
  }
}

class Reading{
  final int timestamp;
  final Data data;
  Reading({
    this.timestamp,
    this.data
  });
  factory Reading.fromJson(Map<String, dynamic> parsedJson) {
    return Reading(
      timestamp: parsedJson['timestamp'],
      data: Data.fromJson(parsedJson['data'])
    );
  }
}

class SetData {
  bool pwr;
  double temp;
  SetData({
    this.pwr,
    this.temp
  });
  factory SetData.fromJson(Map<String, dynamic> json) {
    return SetData(
      pwr: json['pwr'],
      temp: json['temp']
    );
  }
  Map<String, dynamic> toJson() => {
    'pwr': pwr,
    'temp': temp
  };
}

class SendTempSet {
  String topic;
  SetData data;
  SendTempSet({
    this.topic,
    this.data
  });
  factory SendTempSet.fromJson(Map<String, dynamic> json) {
    return SendTempSet(
      topic: json['topic'],
      data: json['data']
    );
  }
  Map<String, dynamic> toJson() => {
    'topic': topic,
    'data': data,
  };
}

Future<http.Response> sendTemp(String send) async {
  final response = await http.post(url, body: send);
  return response;
}

class DataStream extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final title = 'K8S Kitchen Data Stream';
    return MaterialApp(
      title: title,
      home: _DataStream(
        title: title,
        source: SseClient('https://grillbernetes.home.rsmachiner.com/stream/smoker-pi/readings')
      ),
    );
  }
}

class _DataStream extends StatefulWidget {
  final String title;
  final SseClient source;
  _DataStream({Key key, @required this.title, @required this.source})
    : super(key: key);
  
  @override
  __DataStreamState createState() => __DataStreamState();
}

class __DataStreamState extends State<_DataStream> {
  TextEditingController _tempController = new TextEditingController();
  bool _validate = false;
  //final Future<http.Response> sendTemp();
  @override
  Widget build(BuildContext context) {
    final dtf = new DateFormat('HH:mm:ss');
    List readings = new List<Reading>();
    Reading reading;
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.title),
      ),
      body: Padding(
        padding: const EdgeInsets.all(20.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            StreamBuilder(
              stream: widget.source.stream,
              builder: (context, snapshot) {
                if (snapshot.data == null) {
                  return new Text("");
                } else {
                  reading = Reading.fromJson(json.decode(snapshot.data.data));
                  if (readings.length > 20) {
                    readings.removeAt(0);
                  }
                  readings.add(reading);
                  return new Text(
                    "Time: " + dtf.format(new DateTime.now()) + "\n"
                    + "F: " + reading.data.f.toString() + "\n"
                    + "C: " + reading.data.c.toString());
                }
              }
            ),
            TextField(
              controller: _tempController,
              obscureText: false,
              textAlign: TextAlign.left,
              decoration: InputDecoration(
                hintText: 'Set the Temperature',
                hintStyle: TextStyle(color: Colors.grey),
                errorText: _validate ? 'Please enter valid value' : null,
              ),
            ),
            RaisedButton(
              onPressed: () {
                setState(() {
                  var val = _tempController.text;
                  try {
                    var temp = double.parse(val);
                    print(temp.toString());
                    _validate = false;
                    var send = new SendTempSet();
                    send.topic = "smoker-pi-control";
                    send.data = new SetData(
                      pwr: true,
                      temp: temp,
                    );
                    sendTemp(jsonEncode(send));
                  } on Exception catch(e) {
                    _validate = true;
                    print("Invalid temp");
                  }
                });
              },
              child: Text('Set Temp'),
            )
          ]
        )
      ),
    );
  }
}