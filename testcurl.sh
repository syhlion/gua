while true;
do
 time time  curl -X POST http://127.0.0.1:7777/luatest -w %{time_total}\\n\\n
done


