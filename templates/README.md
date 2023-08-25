# Templates

Templates can be used to translate a remote xml into a pdf. It used the logic of [html/template](https://pkg.go.dev/html/template) to loop trough the xml data. The xml data in transformed as data object, therefor we start with creating a processing object from **.data** ``
{{$verzoek:=.data.verzoekXML.verzoek}}``

We have added a custom function marshal that allows you to prittyprint the json object or part of it. ``{{marshal .data}}`` 

We provide a DSO template as inspiration. Where you for example can see how we add scripts to color the marschaled data.