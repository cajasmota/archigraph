module Counter

// Fable + Elmish/Feliz frontend fixture (#5129). A canonical MVU triad with a
// Feliz view, command dispatch, and the Program bootstrap.

open Elmish
open Feliz

type Model =
    { Count: int
      Loading: bool }

type Msg =
    | Increment
    | Decrement
    | Reset
    | Loaded of int

let init () : Model * Cmd<Msg> =
    { Count = 0; Loading = false }, Cmd.ofMsg Reset

let update (msg: Msg) (model: Model) : Model * Cmd<Msg> =
    match msg with
    | Increment -> { model with Count = model.Count + 1 }, Cmd.none
    | Decrement -> { model with Count = model.Count - 1 }, Cmd.none
    | Reset -> { model with Count = 0 }, Cmd.none
    | Loaded n -> { model with Count = n; Loading = false }, Cmd.OfAsync.perform loadData () Loaded

let counterButton (label: string) (onClick: unit -> unit) : ReactElement =
    Html.button [
        prop.text label
        prop.onClick (fun _ -> onClick ())
    ]

let view (model: Model) (dispatch: Msg -> unit) : ReactElement =
    Html.div [
        prop.children [
            Html.h1 [ prop.text (string model.Count) ]
            counterButton "+" (fun () -> dispatch Increment)
            counterButton "-" (fun () -> dispatch Decrement)
        ]
    ]

let main () =
    Program.mkProgram init update view
    |> Program.run
