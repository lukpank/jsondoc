{{title "Example JSON API description"}}

# Example JSON API description

## Request for path `/hello`

Used to obtain greetings for the given name.

{{input "helloInput"}}

{{output "helloOutput"}}

## Request for path `/item/get`

Used to obtain information about the given product.

{{input "itemGetInput"}}

{{output "itemGetOutput"}}

## Request with no fields

{{input "empty"}}
{{input "emptyA"}}
{{input "emptyAA"}}
{{input "emptyO"}}
{{input "emptyOO"}}
{{input "emptyAO"}}
{{input "emptyOA"}}

## Request with maps

{{input "mapInput"}}

{{output "mapOutput"}}

## Request with arrays

{{input "arrayInput"}}

## Request with data from another package

{{input "withAnother"}}

{{output "github.com/lukpank/jsondoc/example/another.Another"}}
