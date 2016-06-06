# swagger-mod_security
Let's imagine a simple API which  ask for a number of tomatoes or potatoes and return the double of this number.  
If we post {"value": "21", "unit": "tomatoes"} to http://server/makeItDouble it'll return {"value": "42", "unit": "tomatoes"} with a status code 200.  
Here is the simplified definition of such an API produced by swagger :  
"paths": {  
  "/makeItDouble": {  
    "post": {  
      "tags": [  
        "MakeItDouble"  
      ],  
      "summary": "/makeItDouble",  
      "description": "doPostMakeItDouble",  
      "operationId": "doPostMakeItDoubleUsingPOST",  
      "consumes": [  
        "application/json"  
	  ],  
      "produces": [  
        "*/*"  
      ],  
      "parameters": [  
        {  
          "name": "value",  
          "in": "query",  
          "description": "valueToDouble",  
          "required": true,  
          "type": "integer",  
          "format": "int32"  
        },  
        {  
          "name": "unit",  
          "in": "query",  
          "description": "Unit",  
          "required": false,  
          "type": "string",  
  		  "enum": [  
            "potatoes",  
            "tomatoes"  
          ]  
        }  
      ],  
      "responses": {  
        "200": {  
          "description": "OK",  
          "schema": {  
            "$ref": "#/definitions/Double"  
          }  
        },  
        "400": {  
          "description": "invalid query"  
        },  
        "401": {  
          "description": "valid query but insert is impossible"  
    	  },  
        "403": {  
          "description": "insert forbidden for this user"  
        },  
        "404": {  
          "description": "Unknown entity"  
        },  
        "500": {  
          "description": "internal server error"  
        }  
      }  
	}  
  }  
},  
"definitions": {  
  "Double": {  
    "properties": {  
	  "value": {  
	    "type": "number"  
	  },  
	  "unit": {  
	    "type": "string",  
		"enum": [  
            "potatoes",  
            "tomatoes"  
        ]  
	  }  
	}  
  }  
}  
  
Here comes the output that'll be generated by SecRuleGen if we supply the URL pointing to this specification file :  
 - First we open a LocationMatch directive, specifying on which path the rules will be applied :  
     <LocationMatch "^/makeItDouble$">  
 
 - A method rule, which will allow only POST and OPTIONS (note : OPTIONS si always accepted by default, because swagger never describe this method) methods on the /makeItDouble endpoint :  
     SecRule REQUEST_METHOD "!^(?:POST|OPTIONS)$" "phase:2,t:none,deny,id:'200',msg:'Unauthorized method',logdata:%{REQUEST_METHOD}"  
	   
 - A rule to exclude given IP adresses from the max rate rules
                 SecRule REMOTE_ADDR "@ipMatch 127.0.0.0/24" "phase:1,skip:4,nolog,id:'201'"
 
 - Maximum rate rules, preventing client to make more than X ( 50 by default ) requests per minute on a given endpoint :
                SecAction "initcol:ip=%{REMOTE_ADDR}_%{TX.uahash},pass,nolog,id:'202'"
                SecAction "phase:5,deprecatevar:ip./makeItDouble=50/60,pass,nolog,id:'203'"
                SecRule IP:/makeItDouble "@gt 50" "phase:2,pause:300,deny,setenv:RATELIMITED,skip:1,id:'204',msg:'too many request per minute',logdata:%{MATCHED_VAR}"
                SecAction "phase:2,pass,setvar:ip./makeItDouble=+1,nolog,id:'205'"
				
 - An ARGS_NAMES rule, allowing only known arguments (value and unit in our case) to be supplied :  
	 SecRule ARGS_NAMES "!^(?:value|unit)$" "phase:2,t:none,deny,id:'206',msg:'Unrecognized argument',logdata:%{ARGS_NAMES}"  
	   
 - An ARGS rule, allowing only valid value to be supplied in a know variable, in our case unit can contain "tomatoes" or "potatoes" :  
     SecRule ARGS:unit "!^(?:tomatoes|potatoes)$" "phase:2,t:none,deny,id:'207',msg:'Bad content',logdata:%{MATCHED_VAR}"  
	   
 - Then we close the LocationMatch section :  
     </LocationMatch>  

