package ru.mitrakov.self.rush.tester

import java.util.UUID

import scala.io.StdIn
import akka.actor._
import akka.routing.RoundRobinPool
import ru.mitrakov.self.rush.tester.protocol.SwUDP


object Starter extends App {
  if (args.length != 3) {
    println(s"Usage java -jar ${System.getProperty("sun.java.command")} <host> <agents_count> <battles_count>")
    System.exit(0)
  }

  private val n = args(1).toInt
  private val network = new Network(args.head, 33996)
  private val system = ActorSystem("highload")
  private val protocol = system.actorOf(Props(new SwUDP(network)), "SwUDP")
  private val workers = system.actorOf(Props(new Worker(protocol)) withRouter RoundRobinPool(10), "workerRouter")
  private val ids = (for (_ <- 0 until n) yield {
    val uuid = UUID.randomUUID()
    (uuid.getLeastSignificantBits + uuid.getMostSignificantBits).toInt
  }).toList
  private val dispatcher = system.actorOf(Props(new Dispatcher(ids, workers)), "dispatcher")

  Agent.battlesPerAgent = args(2).toInt
  network setRouter workers
  protocol ! workers // setting listener
  dispatcher ! "Start timer!"
  ids foreach {workers ! Connect(_)}

  ids.zipWithIndex foreach { w =>
    val (id, i) = w
    val name = f"Test$i%03d"
    Thread sleep 1000 //@mitrakov (2017-08-02): firstly I used bcrypt so delay was 3000, now I use scrypt and delay 1000
    workers ! Start(id, name)
  }

  StdIn.readLine()
  system terminate()
}