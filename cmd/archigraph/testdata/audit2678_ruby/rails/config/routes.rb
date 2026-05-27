Rails.application.routes.draw do
  # Explicit verb route — handler lives in app/controllers/things_controller.rb
  get '/things/search', to: 'things#search'

  # resources macro expands to 7 standard CRUD routes
  resources :things

  # namespace prefixes the path AND the controller module
  namespace :api do
    resources :widgets
  end
end
