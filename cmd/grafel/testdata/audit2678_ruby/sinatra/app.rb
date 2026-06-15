require 'sinatra'

get '/things' do
  content_type :json
  '{"things": []}'
end

post '/things' do
  content_type :json
  '{"stored": true}'
end

get '/things/:id' do
  content_type :json
  "{\"id\": #{params[:id]}}"
end

delete '/things/:id' do
  content_type :json
  '{"deleted": true}'
end
